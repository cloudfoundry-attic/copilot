package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	copilotsnapshot "code.cloudfoundry.org/copilot/snapshot"
	"code.cloudfoundry.org/debugserver"
	"github.com/pivotal-cf/paraphernalia/serve/grpcrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/pkg/mcp/server"
	"istio.io/istio/pkg/mcp/snapshot"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/internalroutes"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/copilot/routes"
	"code.cloudfoundry.org/copilot/vip"
	"code.cloudfoundry.org/lager"
)

func mainWithError() error {
	var configFilePath string
	flag.StringVar(&configFilePath, "config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(configFilePath)
	if err != nil {
		return err
	}

	pilotFacingTLSConfig, err := cfg.ServerTLSConfigForPilot()
	if err != nil {
		return err
	}
	cloudControllerFacingTLSConfig, err := cfg.ServerTLSConfigForCloudController()
	if err != nil {
		return err
	}
	logger := lager.NewLogger("copilot-server")
	reconfigurableSink := lager.NewReconfigurableSink(
		lager.NewWriterSink(os.Stdout, lager.DEBUG),
		lager.INFO)
	logger.RegisterSink(reconfigurableSink)

	debugserver.Run("127.0.0.1:33333", reconfigurableSink)

	var bbsEventer models.BBSEventer
	var diegoTickerChan <-chan time.Time
	if cfg.BBS != nil {
		bbsClient, err := bbs.NewSecureClient(
			cfg.BBS.Address,
			cfg.BBS.ServerCACertPath,
			cfg.BBS.ClientCertPath,
			cfg.BBS.ClientKeyPath,
			cfg.BBS.ClientSessionCacheSize,
			cfg.BBS.MaxIdleConnsPerHost,
		)
		if err != nil {
			return err
		}
		_, err = bbsClient.Cells(logger)
		if err != nil {
			return fmt.Errorf("unable to reach BBS at address %q: %s", cfg.BBS.Address, err)
		}
		diegoTickerChan = time.NewTicker(cfg.BBS.SyncInterval).C
		bbsEventer = bbsClient
	}

	backendSetRepo := models.NewBackendSetRepo(bbsEventer, logger, diegoTickerChan)

	routesRepo := models.NewRoutesRepo()
	routeMappingsRepo := models.NewRouteMappingsRepo()
	capiDiegoProcessAssociationsRepo := &models.CAPIDiegoProcessAssociationsRepo{
		Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
	}

	_, cidr, err := net.ParseCIDR(cfg.VIPCIDR)
	if err != nil {
		return fmt.Errorf("parsing vip cidr: %s", err)
	}

	vipProvider := &vip.Provider{
		CIDR: cidr,
	}
	internalRoutesRepo := &internalroutes.Repo{
		RoutesRepo:                       routesRepo,
		RouteMappingsRepo:                routeMappingsRepo,
		CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
		BackendSetRepo:                   backendSetRepo,
		Logger:                           logger,
		VIPProvider:                      vipProvider,
	}
	istioHandler := &handlers.Istio{
		RoutesRepo:                       routesRepo,
		RouteMappingsRepo:                routeMappingsRepo,
		CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
		BackendSetRepo:                   backendSetRepo,
		Logger:                           logger,
		InternalRoutesRepo:               internalRoutesRepo,
	}
	capiHandler := &handlers.CAPI{
		RoutesRepo:                       routesRepo,
		RouteMappingsRepo:                routeMappingsRepo,
		CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
		Logger:                           logger,
	}
	grpcServerForPilot := grpcrunner.New(logger, cfg.ListenAddressForPilot,
		func(s *grpc.Server) {
			api.RegisterIstioCopilotServer(s, istioHandler)
			reflection.Register(s)
		},
		grpc.Creds(credentials.NewTLS(pilotFacingTLSConfig)),
	)
	grpcServerForCloudController := grpcrunner.New(logger, cfg.ListenAddressForCloudController,
		func(s *grpc.Server) {
			api.RegisterCloudControllerCopilotServer(s, capiHandler)
			reflection.Register(s)
		},
		grpc.Creds(credentials.NewTLS(cloudControllerFacingTLSConfig)),
	)

	// TODO: Remove unsupported typeURLs (everything except Gateway, VirtualService, DestinationRule)
	// when mcp client is capable of only sending supported ones
	typeURLs := []string{
		copilotsnapshot.GatewayTypeURL,
		copilotsnapshot.VirtualServiceTypeURL,
		copilotsnapshot.DestinationRuleTypeURL,
		copilotsnapshot.ServiceEntryTypeURL,
		copilotsnapshot.EnvoyFilterTypeURL,
		copilotsnapshot.HTTPAPISpecTypeURL,
		copilotsnapshot.HTTPAPISpecBindingTypeURL,
		copilotsnapshot.QuotaSpecTypeURL,
		copilotsnapshot.QuotaSpecBindingTypeURL,
		copilotsnapshot.PolicyTypeURL,
		copilotsnapshot.MeshPolicyTypeURL,
		copilotsnapshot.ServiceRoleTypeURL,
		copilotsnapshot.ServiceRoleBindingTypeURL,
		copilotsnapshot.RbacConfigTypeURL,
	}

	cache := snapshot.New(snapshot.DefaultGroupIndex)
	grpcServerForMcp := grpcrunner.New(logger, cfg.ListenAddressForMCP,
		func(s *grpc.Server) {
			authChecker := server.NewAllowAllChecker()
			reporter := server.NewStatsContext("copilot/")
			snapshotServer := server.New(cache, typeURLs, authChecker, reporter)
			mcp.RegisterAggregatedMeshConfigServiceServer(s, snapshotServer)
			reflection.Register(s)
		},
		grpc.Creds(credentials.NewTLS(pilotFacingTLSConfig)),
	)

	mcpTicker := time.NewTicker(cfg.MCPConvergeInterval)
	collector := routes.NewCollector(logger, routesRepo, routeMappingsRepo, capiDiegoProcessAssociationsRepo, backendSetRepo)
	inMemoryBuilder := snapshot.NewInMemoryBuilder()
	mcpSnapshot := copilotsnapshot.New(logger, mcpTicker.C, collector, cache, inMemoryBuilder)
	istioHandler.Collector = collector

	members := grouper.Members{
		grouper.Member{Name: "grpc-server-for-pilot", Runner: grpcServerForPilot},
		grouper.Member{Name: "grpc-server-for-cloud-controller", Runner: grpcServerForCloudController},
		grouper.Member{Name: "grpc-server-for-mcp", Runner: grpcServerForMcp},
		grouper.Member{Name: "mcp-snapshot", Runner: mcpSnapshot},
		grouper.Member{Name: "diego-backend-set-updater", Runner: backendSetRepo},
	}

	group := grouper.NewOrdered(os.Interrupt, members)
	monitor := ifrit.Invoke(sigmon.New(group))
	err = <-monitor.Wait()
	if err != nil {
		return err
	}
	logger.Info("exit")
	return nil
}

func main() {
	err := mainWithError()
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err)
		os.Exit(1)
	}
}
