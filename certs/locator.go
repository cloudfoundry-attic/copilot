package certs

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

type CertChainKeyPair struct {
	CertChain  string
	PrivateKey string
}

type PemInfo struct {
	Hosts    []string
	CertPath string
	KeyPath  string
	Dir      string
}

//go:generate counterfeiter -o fakes/locator.go --fake-name Locator . Librarian
type Librarian interface {
	Locate() ([]PemInfo, error)
	Stow() error
}

type Locator struct {
	destinationDir string
	pairs          []CertChainKeyPair
}

func NewLocator(destinationDir string, pairs []CertChainKeyPair) *Locator {
	return &Locator{
		destinationDir: destinationDir,
		pairs:          pairs,
	}
}

func (l *Locator) Locate() (paths []PemInfo, err error) {
	for _, pair := range l.pairs {
		certPem := []byte(pair.CertChain)

		pairPaths, err := l.createPemInfo(certPem)
		if err != nil {
			return paths, err
		}

		paths = append(paths, pairPaths)
	}

	return paths, nil
}

func (l *Locator) Stow() error {
	for _, pair := range l.pairs {
		certPem := []byte(pair.CertChain)
		keyPem := []byte(pair.PrivateKey)

		paths, err := l.createPemInfo(certPem)
		if err != nil {
			return err
		}

		dir := paths.Dir
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err = os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				return err
			}
		}

		err = ioutil.WriteFile(paths.CertPath, certPem, 0600)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(paths.KeyPath, keyPem, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *Locator) createPemInfo(certPem []byte) (paths PemInfo, err error) {
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

	dirPath := fmt.Sprintf("%s/%s", l.destinationDir, hostnames[0])
	paths = PemInfo{
		Hosts:    hostnames,
		Dir:      dirPath,
		CertPath: fmt.Sprintf("%s/tls.crt", dirPath),
		KeyPath:  fmt.Sprintf("%s/tls.key", dirPath),
	}

	return paths, nil
}
