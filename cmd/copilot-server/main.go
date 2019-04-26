package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/certs"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/copilot/routes"
	copilotsnapshot "code.cloudfoundry.org/copilot/snapshot"
	"code.cloudfoundry.org/copilot/vip"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/paraphernalia/serve/grpcrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // enable GZIP compression on the server side
	"google.golang.org/grpc/reflection"

	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/galley/pkg/runtime/groups"
	"istio.io/istio/pkg/log"
	"istio.io/istio/pkg/mcp/monitoring"
	"istio.io/istio/pkg/mcp/server"
	"istio.io/istio/pkg/mcp/snapshot"
	"istio.io/istio/pkg/mcp/source"
)

const istioCertRootPath = "/etc/istio"

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

	var copilotLogLevel lager.LogLevel
	switch cfg.LogLevel {
	case "debug":
		copilotLogLevel = lager.DEBUG
	case "info":
		copilotLogLevel = lager.INFO
	case "error":
		copilotLogLevel = lager.ERROR
	case "fatal":
		copilotLogLevel = lager.FATAL
	}
	reconfigurableSink := lager.NewReconfigurableSink(
		lager.NewWriterSink(os.Stdout, lager.DEBUG),
		copilotLogLevel)
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
		diegoTickerChan = time.NewTicker(time.Duration(cfg.BBS.SyncInterval)).C
		bbsEventer = bbsClient
	}

	backendSetRepo := models.NewBackendSetRepo(bbsEventer, logger, diegoTickerChan)

	routesRepo := models.NewRoutesRepo()
	routeMappingsRepo := models.NewRouteMappingsRepo()
	capiDiegoProcessAssociationsRepo := &models.CAPIDiegoProcessAssociationsRepo{
		Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
	}
	vipCidr, err := cfg.GetVIPCIDR()
	if err != nil {
		return err
	}
	vipProvider := vip.NewProvider(vipCidr)

	capiHandler := &handlers.CAPI{
		RoutesRepo:                       routesRepo,
		RouteMappingsRepo:                routeMappingsRepo,
		CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
		VIPProvider:                      vipProvider,
		Logger:                           logger,
	}
	grpcServerForCloudController := grpcrunner.New(logger, cfg.ListenAddressForCloudController,
		func(s *grpc.Server) {
			api.RegisterCloudControllerCopilotServer(s, capiHandler)
			reflection.Register(s)
		},
		grpc.Creds(credentials.NewTLS(cloudControllerFacingTLSConfig)),
	)

	vipResolverHandler := &handlers.VIPResolver{
		RoutesRepo: routesRepo,
		Logger:     logger,
	}
	grpcServerForVIPResolver := grpcrunner.New(logger, cfg.ListenAddressForVIPResolver,
		func(s *grpc.Server) {
			api.RegisterVIPResolverCopilotServer(s, vipResolverHandler)
			reflection.Register(s)
		},
	)

	// TODO: Remove unsupported typeURLs (everything except Gateway, VirtualService, DestinationRule)
	// when mcp client is capable of only sending supported ones
	typeURLs := []string{
		copilotsnapshot.GatewayTypeURL,
		copilotsnapshot.VirtualServiceTypeURL,
		copilotsnapshot.DestinationRuleTypeURL,
		copilotsnapshot.SidecarTypeURL,
		copilotsnapshot.PolicyTypeURL,
		copilotsnapshot.ServiceEntryTypeURL,
	}

	collectionOptions := source.CollectionOptionsFromSlice(typeURLs)

	cache := snapshot.New(groups.IndexFunction)
	grpcServerForMcp := grpcrunner.New(logger, cfg.ListenAddressForMCP,
		func(s *grpc.Server) {
			authChecker := server.NewAllowAllChecker()
			reporter := monitoring.NewStatsContext("copilot/")
			options := &source.Options{
				Watcher:           cache,
				Reporter:          reporter,
				CollectionOptions: collectionOptions,
			}
			// TODO: Figure out sane NewConnectionsFreq and NewConnectionsBurstSize when we're doing scaling work.
			// (https://www.pivotaltracker.com/story/show/162515083)
			serverOptions := &source.ServerOptions{
				NewConnectionFreq:      1000000000,
				NewConnectionBurstSize: 1000000000,
				AuthChecker:            authChecker,
			}

			mcpServer := source.NewServer(options, serverOptions)
			var pilotLogLevel log.Level
			switch cfg.LogLevel {
			case "debug":
				pilotLogLevel = log.DebugLevel
			case "info":
				pilotLogLevel = log.InfoLevel
			case "error":
				pilotLogLevel = log.ErrorLevel
			case "fatal":
				pilotLogLevel = log.FatalLevel
			}
			for name, scope := range log.Scopes() {
				scope.SetOutputLevel(pilotLogLevel)
				logger.Info("set pilot log level for scope", lager.Data{"scope-name": name})
			}

			mcp.RegisterResourceSourceServer(s, mcpServer)
			reflection.Register(s)
		},
		grpc.Creds(credentials.NewTLS(pilotFacingTLSConfig)),
	)

	mcpTicker := time.NewTicker(time.Duration(cfg.MCPConvergeInterval))
	collector := routes.NewCollector(logger, routesRepo, routeMappingsRepo, capiDiegoProcessAssociationsRepo, backendSetRepo, vipProvider)
	inMemoryBuilder := snapshot.NewInMemoryBuilder()
	librarian := certs.NewLocator(istioCertRootPath, cfg.TLSPems)
	snapshotConfig := copilotsnapshot.NewConfig(librarian, logger)
	mcpSnapshot := copilotsnapshot.New(logger, mcpTicker.C, collector, cache, inMemoryBuilder, snapshotConfig)

	members := grouper.Members{
		grouper.Member{Name: "grpc-server-for-cloud-controller", Runner: grpcServerForCloudController},
		grouper.Member{Name: "grpc-server-for-vip-resolver", Runner: grpcServerForVIPResolver},
		grouper.Member{Name: "grpc-server-for-mcp", Runner: grpcServerForMcp},
		grouper.Member{Name: "mcp-snapshot", Runner: mcpSnapshot},
		grouper.Member{Name: "diego-backend-set-updater", Runner: backendSetRepo},
	}

	group := grouper.NewOrdered(os.Interrupt, members)
	monitor := ifrit.Invoke(sigmon.New(group, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT))
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
