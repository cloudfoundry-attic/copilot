package librarian

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"code.cloudfoundry.org/copilot/certs"
	validator "gopkg.in/validator.v2"
)

type Config struct {
	IstioCertRootPath string
	TLSPems           []certs.CertChainKeyPair
}

const DefaultIstioCertRootPath = "/etc/istio"

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

	if c.IstioCertRootPath == "" {
		c.IstioCertRootPath = DefaultIstioCertRootPath
	}

	return c, nil
}
