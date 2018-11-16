package certs

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

type CertChainKeyPair struct {
	CertChain  string
	PrivateKey string
}

type CertPairPaths struct {
	Hosts    []string
	CertPath string
	KeyPath  string
}

//go:generate counterfeiter -o fakes/locator.go --fake-name Locator . Librarian
type Librarian interface {
	Locate() ([]CertPairPaths, error)
	Stow() error
}

type Locator struct {
	pairs []CertChainKeyPair
}

func NewLocator(pairs []CertChainKeyPair) *Locator {
	return &Locator{
		pairs: pairs,
	}
}

func (l *Locator) Locate() (paths []CertPairPaths, err error) {
	for _, pair := range l.pairs {
		certPem := []byte(pair.CertChain)

		block, _ := pem.Decode(certPem)
		if block == nil {
			return paths, errors.New("failed to decode cert pem")
		}

		certs, err := x509.ParseCertificates(block.Bytes)
		if err != nil {
			return paths, err
		}

		hostnames := []string{}
		for _, cert := range certs {
			hostnames = append(hostnames, cert.DNSNames...)
		}

		if len(hostnames) == 0 {
			return paths, errors.New("no DNS names provided in certificates")
		}

		stuff := CertPairPaths{
			Hosts:    hostnames,
			CertPath: fmt.Sprintf("/etc/istio/%s/tls.crt", hostnames[0]),
			KeyPath:  fmt.Sprintf("/etc/istio/%s/tls.key", hostnames[0]),
		}

		paths = append(paths, stuff)
	}

	return paths, nil
}

func (l *Locator) Stow() error {
	fmt.Printf("not implemented")
	return nil
}
