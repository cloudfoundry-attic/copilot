package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/copilot"
	"code.cloudfoundry.org/copilot/api"
)

func mainWithError() error {
	var address, caCert, clientCert, clientKey string

	flag.StringVar(&address, "address", "127.0.0.1:9000", "ip:port of copilot server")
	flag.StringVar(&caCert, "server-ca", "", "path to cert for the copilot server CA")
	flag.StringVar(&clientCert, "client-cert", "", "path to cert for the copilot client")
	flag.StringVar(&clientKey, "client-key", "", "path to key for the copilot client")

	flag.Parse()

	if address == "" || caCert == "" || clientCert == "" || clientKey == "" {
		flag.PrintDefaults()
		return errors.New("all flags are required")
	}

	positionalArgs := flag.Args()
	if len(positionalArgs) < 1 || (positionalArgs[0] != "health" && positionalArgs[0] != "routes") {
		return errors.New(`must provide one of the following subcommands: [health, routes]`)
	}

	caCertBytes, err := ioutil.ReadFile(caCert)
	if err != nil {
		return fmt.Errorf("reading ca cert file: %s", err)
	}

	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(caCertBytes); !ok {
		return errors.New("parsing server CAs: invalid pem block")
	}

	tlsCert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		return fmt.Errorf("loading client cert/key: %s", err)
	}

	tlsConfig := &tls.Config{
		RootCAs:      rootCAs,
		Certificates: []tls.Certificate{tlsCert},
	}

	client, err := copilot.NewClient(address, tlsConfig)
	if err != nil {
		return fmt.Errorf("copilot client: %s", err)
	}

	switch positionalArgs[0] {
	case "health":
		resp, err := client.Health(context.Background(), new(api.HealthRequest))
		if err != nil {
			return fmt.Errorf("copilot health request: %s", err)
		}
		fmt.Println("Copilot Health Response:", resp.GetHealthy())
	case "routes":
		resp, err := client.Routes(context.Background(), new(api.RoutesRequest))
		if err != nil {
			return fmt.Errorf("copilot health request: %s", err)
		}
		fmt.Println("Copilot Routes Response:", resp.GetBackends())
	}

	return nil
}

func main() {
	err := mainWithError()
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error running copilot client: %s\n", err)
		os.Exit(1)
	}
}
