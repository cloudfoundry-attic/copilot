package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/grpc_server"
	"github.com/tedsuo/ifrit/sigmon"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
)

type CopilotHandler struct{}

func (c *CopilotHandler) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	return &api.HealthResponse{Healthy: true}, nil
}

func main() {
	var configFilePath string
	flag.StringVar(&configFilePath, "config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(configFilePath)
	if err != nil {
		panic(err)
	}

	copilotHandler := &CopilotHandler{}

	tlsConfig, err := cfg.ServerTLSConfig()
	if err != nil {
		panic(err)
	}
	grpcServer := grpc_server.NewGRPCServer(":8888", tlsConfig, copilotHandler, api.RegisterCopilotServer)
	members := grouper.Members{
		grouper.Member{Name: "gprc-server", Runner: grpcServer},
		grouper.Member{Name: "dummy", Runner: ifrit.RunFunc(dummyRunner)},
	}
	group := grouper.NewOrdered(os.Interrupt, members)
	process := ifrit.Invoke(sigmon.New(group))
	errChan := process.Wait()
	err = <-errChan
	if err != nil {
		panic(err)
	}
	fmt.Println("exited")
}

func dummyRunner(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	fmt.Println("Copilot started")
	select {
	case <-signals:
		fmt.Println("exiting")
		return nil
	}
}
