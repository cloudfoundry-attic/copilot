package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"gopkg.in/validator.v2"
)

type BBSConfig struct {
	ServerCACertPath       string `validate:"nonzero"`
	ClientCertPath         string `validate:"nonzero"`
	ClientKeyPath          string `validate:"nonzero"`
	Address                string `validate:"nonzero"`
	ClientSessionCacheSize int
	MaxIdleConnsPerHost    int
}

type Config struct {
	ListenAddressForPilot           string `validate:"nonzero"`
	ListenAddressForCloudController string `validate:"nonzero"`
	PilotClientCAPath               string `validate:"nonzero"`
	CloudControllerClientCAPath     string `validate:"nonzero"`
	ServerCertPath                  string `validate:"nonzero"`
	ServerKeyPath                   string `validate:"nonzero"`

	BBS BBSConfig
}

func (c *Config) Save(path string) error {
	configBytes, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, configBytes, 0600)
}

func Load(path string) (*Config, error) {
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := new(Config)
	err = json.Unmarshal(configBytes, c)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %s", err)
	}
	err = validator.Validate(c)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %s", err)
	}
	return c, nil
}

func (c *Config) ServerTLSConfigForPilot() (*tls.Config, error) {
	return c.serverTLSConfigForClient("pilot", c.PilotClientCAPath)
}

func (c *Config) ServerTLSConfigForCloudController() (*tls.Config, error) {
	return c.serverTLSConfigForClient("cloud controller", c.CloudControllerClientCAPath)
}

func (c *Config) serverTLSConfigForClient(clientName string, clientCAPath string) (*tls.Config, error) {
	serverCert, err := tls.LoadX509KeyPair(c.ServerCertPath, c.ServerKeyPath)
	if err != nil {
		return nil, fmt.Errorf("parsing %s-facing server cert/key: %s", clientName, err)
	}

	clientCABytes, err := ioutil.ReadFile(clientCAPath)
	if err != nil {
		return nil, fmt.Errorf("loading client CAs for %s-facing server: %s", clientName, err)
	}
	clientCAs := x509.NewCertPool()
	if ok := clientCAs.AppendCertsFromPEM(clientCABytes); !ok {
		return nil, fmt.Errorf("parsing client CAs for %s-facing server: invalid pem block", clientName)
	}

	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		CurvePreferences: []tls.CurveID{tls.CurveP384},
		ClientAuth:       tls.RequireAndVerifyClientCert,
		Certificates:     []tls.Certificate{serverCert},
		ClientCAs:        clientCAs,
	}, nil
}
