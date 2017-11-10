package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"gopkg.in/validator.v2"
)

type Config struct {
	ListenAddress string `validate:"nonzero"`
	ClientCA      string `validate:"nonzero"`
	ServerCert    string `validate:"nonzero"`
	ServerKey     string `validate:"nonzero"`
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

func (c *Config) ServerTLSConfig() (*tls.Config, error) {
	serverCert, err := tls.X509KeyPair([]byte(c.ServerCert), []byte(c.ServerKey))
	if err != nil {
		return nil, fmt.Errorf("parsing server cert/key: %s", err)
	}

	clientCAs := x509.NewCertPool()
	if ok := clientCAs.AppendCertsFromPEM([]byte(c.ClientCA)); !ok {
		return nil, errors.New("parsing client CAs: invalid pem block")
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
