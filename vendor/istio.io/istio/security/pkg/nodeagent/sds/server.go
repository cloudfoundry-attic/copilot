// Copyright 2018 Istio Authors
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

package sds

import (
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"istio.io/istio/pkg/log"
	"istio.io/istio/security/pkg/nodeagent/cache"
)

const maxStreams = 100000

// Options provides all of the configuration parameters for secret discovery service.
type Options struct {
	// UDSPath is the unix domain socket through which SDS server communicates with proxies.
	UDSPath string

	// CertFile is the path of Cert File for gRPC server TLS settings.
	CertFile string

	// KeyFile is the path of Key File for gRPC server TLS settings.
	KeyFile string

	// CAEndpoint is the CA endpoint to which node agent sends CSR request.
	CAEndpoint string

	// The CA provider name.
	CAProviderName string
}

// Server is the gPRC server that exposes SDS through UDS.
type Server struct {
	envoySds *sdsservice

	grpcListener net.Listener
	grpcServer   *grpc.Server
}

// NewServer creates and starts the Grpc server for SDS.
func NewServer(options Options, st cache.SecretManager) (*Server, error) {
	s := &Server{
		envoySds: newSDSService(st),
	}
	if err := s.initDiscoveryService(&options, st); err != nil {
		log.Errorf("Failed to initialize secret discovery service: %v", err)
		return nil, err
	}

	log.Infof("SDS gRPC server start, listen %q \n", options.UDSPath)

	return s, nil
}

// Stop closes the gRPC server.
func (s *Server) Stop() {
	if s == nil {
		return
	}

	if s.grpcListener != nil {
		s.grpcListener.Close()
	}

	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
}

func (s *Server) initDiscoveryService(options *Options, st cache.SecretManager) error {
	s.grpcServer = grpc.NewServer(s.grpcServerOptions(options)...)
	s.envoySds.register(s.grpcServer)

	// Remove unix socket before use.
	if err := os.Remove(options.UDSPath); err != nil && !os.IsNotExist(err) {
		// Anything other than "file not found" is an error.
		log.Errorf("Failed to remove unix://%s: %v", options.UDSPath, err)
		return fmt.Errorf("failed to remove unix://%s", options.UDSPath)
	}

	var err error
	s.grpcListener, err = net.Listen("unix", options.UDSPath)
	if err != nil {
		log.Errorf("Failed to listen on unix socket %q: %v", options.UDSPath, err)
		return err
	}

	// Update SDS UDS file permission so that istio-proxy has permission to access it.
	if _, err := os.Stat(options.UDSPath); err != nil {
		log.Errorf("SDS uds file %q doesn't exist", options.UDSPath)
		return fmt.Errorf("sds uds file %q doesn't exist", options.UDSPath)
	}
	if err := os.Chmod(options.UDSPath, 0666); err != nil {
		log.Errorf("Failed to update %q permission", options.UDSPath)
		return fmt.Errorf("failed to update %q permission", options.UDSPath)
	}

	go func() {
		if err = s.grpcServer.Serve(s.grpcListener); err != nil {
			log.Errorf("SDS grpc server failed to start: %v", err)
		}

		log.Info("SDS grpc server started")
	}()

	return nil
}

func (s *Server) grpcServerOptions(options *Options) []grpc.ServerOption {
	grpcOptions := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(maxStreams)),
	}

	if options.CertFile != "" && options.KeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(options.CertFile, options.KeyFile)
		if err != nil {
			log.Errorf("Failed to load TLS keys: %s", err)
			return nil
		}
		grpcOptions = append(grpcOptions, grpc.Creds(creds))
	}

	return grpcOptions
}
