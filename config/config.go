package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"time"

	"code.cloudfoundry.org/copilot/certs"
	"code.cloudfoundry.org/durationjson"
	validator "gopkg.in/validator.v2"
)

type BBSConfig struct {
	ServerCACertPath       string `validate:"nonzero"`
	ClientCertPath         string `validate:"nonzero"`
	ClientKeyPath          string `validate:"nonzero"`
	Address                string `validate:"nonzero"`
	ClientSessionCacheSize int
	MaxIdleConnsPerHost    int
	Disable                bool
	SyncInterval           durationjson.Duration
}

const DefaultBBSSyncInterval = durationjson.Duration(60 * time.Second)
const DefaultMCPConvergeInterval = durationjson.Duration(30 * time.Second)

type Config struct {
	ListenAddressForCloudController string `validate:"nonzero"`
	ListenAddressForMCP             string `validate:"nonzero"`
	PilotClientCAPath               string `validate:"nonzero"`
	CloudControllerClientCAPath     string `validate:"nonzero"`
	ServerCertPath                  string `validate:"nonzero"`
	ServerKeyPath                   string `validate:"nonzero"`
	VIPCIDR                         string `validate:"cidr"`
	MCPConvergeInterval             durationjson.Duration

	BBS     *BBSConfig
	TLSPems []certs.CertChainKeyPair
}

func init() {
	validator.SetValidationFunc("cidr", validateCIDR)
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
	if c.BBS == nil {
		return nil, errors.New("invalid config: missing required 'BBS' field")
	}
	if c.BBS.SyncInterval == 0 {
		c.BBS.SyncInterval = DefaultBBSSyncInterval
	}
	if c.BBS.Disable {
		c.BBS = nil // a hack to skip validating BBS fields if user explicitly disables BBS
	}
	if c.MCPConvergeInterval == 0 {
		c.MCPConvergeInterval = durationjson.Duration(DefaultMCPConvergeInterval)
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

func (c *Config) GetVIPCIDR() (*net.IPNet, error) {
	_, cidr, err := net.ParseCIDR(c.VIPCIDR)
	return cidr, err
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

	// these magic values are copied from
	//   https://github.com/pivotal-cf/paraphernalia/blob/4272315231ce0d2636eeb44ed0479e56ca165581/secure/tlsconfig/config.go#L71-L94
	// with a tweak: we relax the curve preferences constraint in order to interoperate
	// with the Ruby gRPC client library:
	//  https://github.com/grpc/grpc/blob/633add81614f9d3877a2b3980ba99d0c9e8c687d/src/core/tsi/ssl_transport_security.cc#L641
	// TODO: follow-up with Security team to determine if we care
	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCAs,
	}, nil
}

func validateCIDR(v interface{}, param string) error {
	st := reflect.ValueOf(v)
	if st.Kind() != reflect.String {
		return validator.ErrUnsupported
	}

	_, _, err := net.ParseCIDR(st.String())
	if err != nil {
		return err
	}

	return nil
}
