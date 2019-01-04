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
package mcp

import (
	"fmt"
	"log"
	"net"
	"net/url"

	"google.golang.org/grpc"

	mcp "istio.io/api/mcp/v1alpha1"
	mcpserver "istio.io/istio/pkg/mcp/server"
	"istio.io/istio/pkg/mcp/testing/monitoring"
)

type WatchResponse func(req *mcp.MeshConfigRequest) (*mcpserver.WatchResponse, mcpserver.CancelWatchFunc)

type mockWatcher struct {
	response WatchResponse
}

func (m mockWatcher) Watch(req *mcp.MeshConfigRequest, pushResponse mcpserver.PushResponseFunc) mcpserver.CancelWatchFunc {
	response, cancel := m.response(req)
	pushResponse(response)
	return cancel
}

type Server struct {
	// The internal snapshot.Cache that the server is using.
	Watcher *mockWatcher

	// TypeURLs that were originally passed in.
	TypeURLs []string

	// Port that the service is listening on.
	Port int

	// The gRPC compatible address of the service.
	URL *url.URL

	gs *grpc.Server
	l  net.Listener
}

func NewServer(typeUrls []string, watchResponseFunc WatchResponse) (*Server, error) {
	watcher := mockWatcher{
		response: watchResponseFunc,
	}
	s := mcpserver.New(watcher, typeUrls, mcpserver.NewAllowAllChecker(), mcptestmon.NewInMemoryServerStatsContext())

	l, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return nil, err
	}

	p := l.Addr().(*net.TCPAddr).Port

	u, err := url.Parse(fmt.Sprintf("tcp://localhost:%d", p))
	if err != nil {
		_ = l.Close()
		return nil, err
	}

	gs := grpc.NewServer()

	mcp.RegisterAggregatedMeshConfigServiceServer(gs, s)
	go func() { _ = gs.Serve(l) }()
	log.Printf("MCP mock server listening on localhost:%d", p)

	return &Server{
		Watcher:  &watcher,
		TypeURLs: typeUrls,
		Port:     p,
		URL:      u,
		gs:       gs,
		l:        l,
	}, nil
}

func (t *Server) Close() (err error) {
	if t.gs != nil {
		t.gs.Stop()
		t.gs = nil
	}

	t.l = nil // gRPC stack will close this
	t.Watcher = nil
	t.TypeURLs = nil
	t.Port = 0

	return
}
