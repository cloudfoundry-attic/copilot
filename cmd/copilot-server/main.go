package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/pivotal-cf/paraphernalia/serve/grpcrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/internalroutes"
	"code.cloudfoundry.org/copilot/models"
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

	var bbsClient bbs.InternalClient
	diegoBulkSyncInterval := 60 * time.Second
	if cfg.BBS == nil {
		logger.Info("BBS is disabled")
		bbsClient = nil
	} else {
		if cfg.BBS.SyncInterval != "" {
			diegoBulkSyncInterval, err = time.ParseDuration(cfg.BBS.SyncInterval)
			if err != nil {
				return err
			}
		}

		bbsClient, err = bbs.NewSecureClient(
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
	}

	routesRepo := models.NewRoutesRepo()
	capiDiegoProcessAssociationsRepo := &models.CAPIDiegoProcessAssociationsRepo{
		Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
	}

	t := time.NewTicker(diegoBulkSyncInterval)
	backendSetRepo := models.NewBackendSetRepo(bbsClient, logger, t.C)

	_, cidr, err := net.ParseCIDR(cfg.VIPCIDR)
	if err != nil {
		return fmt.Errorf("parsing vip cidr: %s", err)
	}

	vipProvider := &vip.Provider{
		CIDR: cidr,
	}
	internalRoutesRepo := &internalroutes.Repo{
		RoutesRepo:                       routesRepo,
		CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
		BBSClient:                        bbsClient,
		Logger:                           logger,
		VIPProvider:                      vipProvider,
	}
	istioHandler := &handlers.Istio{
		RoutesRepo:                       routesRepo,
		CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
		BackendSetRepo:                   backendSetRepo,
		Logger:                           logger,
		InternalRoutesRepo:               internalRoutesRepo,
	}
	capiHandler := &handlers.CAPI{
		RoutesRepo:                       routesRepo,
		CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
		Logger: logger,
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

	members := grouper.Members{
		grouper.Member{Name: "gprc-server-for-pilot", Runner: grpcServerForPilot},
		grouper.Member{Name: "gprc-server-for-cloud-controller", Runner: grpcServerForCloudController},
	}
	if bbsClient != nil {
		members = append(members, grouper.Member{Name: "diego-backend-set-updater", Runner: backendSetRepo})
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
