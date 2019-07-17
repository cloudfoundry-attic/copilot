// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package platform

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"istio.io/istio/pkg/spiffe"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"strings"

	"istio.io/istio/security/pkg/pki/util"
)

// OnPremClientImpl is the implementation of on premise metadata client.
type OnPremClientImpl struct {
	// Root CA cert file to validate the gRPC service in CA.
	rootCertFile string
	// The private key file
	keyFile string
	// The cert chain file
	certChainFile string
}

// CitadelDNSSan is the hardcoded DNS SAN used to identify citadel server.
// The user may use an IP address to connect to the mesh.
const CitadelDNSSan = "istio-citadel"

// NewOnPremClientImpl creates a new OnPremClientImpl.
func NewOnPremClientImpl(rootCert, key, certChain string) (*OnPremClientImpl, error) {
	if _, err := os.Stat(rootCert); err != nil {
		return nil, fmt.Errorf("failed to create onprem client root cert file %v error %v", rootCert, err)
	}
	if _, err := os.Stat(key); err != nil {
		return nil, fmt.Errorf("failed to create onprem client key file %v", err)
	}
	if _, err := os.Stat(certChain); err != nil {
		return nil, fmt.Errorf("failed to create onprem client certChain file %v", err)
	}
	return &OnPremClientImpl{rootCert, key, certChain}, nil
}

// GetDialOptions returns the GRPC dial options to connect to the CA.
func (ci *OnPremClientImpl) GetDialOptions() ([]grpc.DialOption, error) {
	transportCreds, err := getTLSCredentials(ci.rootCertFile,
		ci.keyFile, ci.certChainFile)
	if err != nil {
		return nil, err
	}

	var options []grpc.DialOption
	options = append(options, grpc.WithTransportCredentials(transportCreds))
	return options, nil
}

// IsProperPlatform returns whether the platform is on premise.
func (ci *OnPremClientImpl) IsProperPlatform() bool {
	return true
}

// GetServiceIdentity gets the service account from the cert SAN field.
func (ci *OnPremClientImpl) GetServiceIdentity() (string, error) {
	certBytes, err := ioutil.ReadFile(ci.certChainFile)
	if err != nil {
		return "", err
	}
	cert, err := util.ParsePemEncodedCertificate(certBytes)
	if err != nil {
		return "", err
	}
	serviceIDs, err := util.ExtractIDs(cert.Extensions)
	if err != nil {
		return "", err
	}
	if len(serviceIDs) != 1 {
		for _, s := range serviceIDs {
			if strings.HasPrefix(s, spiffe.URIPrefix) {
				return s, nil
			}
		}
		return "", fmt.Errorf("cert does not have siffe:// SAN fields")
	}
	return serviceIDs[0], nil
}

// GetAgentCredential passes the certificate to control plane to authenticate
func (ci *OnPremClientImpl) GetAgentCredential() ([]byte, error) {
	certBytes, err := ioutil.ReadFile(ci.certChainFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %s", ci.certChainFile)
	}
	return certBytes, nil
}

// GetCredentialType returns "onprem".
func (ci *OnPremClientImpl) GetCredentialType() string {
	return "onprem"
}

// getTLSCredentials creates transport credentials that are common to
// node agent and CA.
func getTLSCredentials(rootCertFile, keyFile, certChainFile string) (credentials.TransportCredentials, error) {

	// Load the certificate from disk
	certificate, err := tls.LoadX509KeyPair(certChainFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot load key pair: %s", err)
	}

	// Create a certificate pool
	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(rootCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %s", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, fmt.Errorf("failed to append certificates")
	}

	config := tls.Config{
		Certificates: []tls.Certificate{certificate},
	}
	config.RootCAs = certPool
	config.ServerName = CitadelDNSSan

	return credentials.NewTLS(&config), nil
}
