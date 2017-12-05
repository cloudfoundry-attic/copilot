package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pivotal-cf/paraphernalia/serve/grpcrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/handlers"
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

	tlsConfig, err := cfg.ServerTLSConfig()
	if err != nil {
		return err
	}
	logger := lager.NewLogger("copilot-server")
	reconfigurableSink := lager.NewReconfigurableSink(
		lager.NewWriterSink(os.Stdout, lager.DEBUG),
		lager.INFO)
	logger.RegisterSink(reconfigurableSink)

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

	handler := &handlers.Copilot{
		RoutesRepo: make(map[string]*handlers.RouteMapping),
		BBSClient:  bbsClient,
		Logger:     logger,
	}
	grpcServer := grpcrunner.New(logger, cfg.ListenAddress,
		func(s *grpc.Server) { api.RegisterIstioCopilotServer(s, handler) },
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	)

	members := grouper.Members{
		grouper.Member{Name: "gprc-server", Runner: grpcServer},
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
