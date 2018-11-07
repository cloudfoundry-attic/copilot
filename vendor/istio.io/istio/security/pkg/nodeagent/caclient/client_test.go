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

package caclient

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"

	gcapb "istio.io/istio/security/proto/providers/google"
)

const mockServerAddress = "localhost:0"

var (
	fakeCert  = []string{"foo", "bar"}
	fakeToken = "Bearer fakeToken"
)

type mockCAServer struct{}

func (ca *mockCAServer) CreateCertificate(ctx context.Context, in *gcapb.IstioCertificateRequest) (*gcapb.IstioCertificateResponse, error) {
	return &gcapb.IstioCertificateResponse{CertChain: fakeCert}, nil
}

func TestCAClient(t *testing.T) {
	// create a local grpc server
	s := grpc.NewServer()
	defer s.Stop()
	lis, err := net.Listen("tcp", mockServerAddress)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	serv := mockCAServer{}

	go func() {
		gcapb.RegisterIstioCertificateServiceServer(s, &serv)
		if err := s.Serve(lis); err != nil {
			t.Fatalf("failed to serve: %v", err)
		}
	}()

	// The goroutine starting the server may not be ready, results in flakiness.
	time.Sleep(1 * time.Second)

	cli, err := NewCAClient(lis.Addr().String(), googleCA, false)
	if err != nil {
		t.Fatalf("failed to create ca client: %v", err)
	}

	resp, err := cli.CSRSign(context.Background(), []byte{01}, fakeToken, 1)
	if err != nil {
		t.Fatalf("failed to call CSR sign: %v", err)
	}

	if !reflect.DeepEqual(resp, fakeCert) {
		t.Errorf("resp: got %+v, expected %q", resp, fakeCert)
	}
}
