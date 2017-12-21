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
	var address, caCert, clientCert, clientKey, hostname, processGUID string

	flag.StringVar(&address, "address", "127.0.0.1:9000", "ip:port of copilot server")
	flag.StringVar(&caCert, "server-ca", "", "path to cert for the copilot server CA")
	flag.StringVar(&clientCert, "client-cert", "", "path to cert for the copilot client")
	flag.StringVar(&clientKey, "client-key", "", "path to key for the copilot client")
	flag.StringVar(&hostname, "hostname", "", "hostname in route to add (e.g. foo.example.com)")
	flag.StringVar(&processGUID, "process-guid", "", "process guid for route to add")

	flag.Parse()

	if address == "" || caCert == "" || clientCert == "" || clientKey == "" {
		flag.PrintDefaults()
		return errors.New("missing one of the following required flags: [address, caCert, clientCert, clientKey]")
	}

	positionalArgs := flag.Args()
	if len(positionalArgs) < 1 || (positionalArgs[0] != "health" && positionalArgs[0] != "add-route") {
		return errors.New(`must provide one of the following subcommands: [health, add-route]`)
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

	client, err := copilot.NewCloudControllerClient(address, tlsConfig)
	if err != nil {
		return fmt.Errorf("copilot client: %s", err)
	}

	switch positionalArgs[0] {
	case "add-route":
		_, err := client.AddRoute(context.Background(), &api.AddRequest{
			ProcessGuid: processGUID,
			Hostname:    hostname,
		})
		if err != nil {
			return fmt.Errorf("copilot add-route request: %s", err)
		}
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
