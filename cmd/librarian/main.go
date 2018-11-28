package main

import (
	"flag"
	"fmt"
	"os"

	"code.cloudfoundry.org/copilot/certs"
	librarian "code.cloudfoundry.org/copilot/librarianconfig"
	"code.cloudfoundry.org/lager"
	_ "google.golang.org/grpc/encoding/gzip" // enable GZIP compression on the server side
)

func mainWithError() error {
	var configFilePath string
	flag.StringVar(&configFilePath, "config", "", "path to config file")
	flag.Parse()

	cfg, err := librarian.Load(configFilePath)
	if err != nil {
		return err
	}

	logger := lager.NewLogger("librarian")
	reconfigurableSink := lager.NewReconfigurableSink(
		lager.NewWriterSink(os.Stdout, lager.DEBUG),
		lager.INFO)
	logger.RegisterSink(reconfigurableSink)

	librarian := certs.NewLocator(cfg.IstioCertRootPath, cfg.TLSPems)
	logger.Info("stowing certs")
	err = librarian.Stow()
	if err != nil {
		return err
	}
	logger.Info("certs stowed")

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
