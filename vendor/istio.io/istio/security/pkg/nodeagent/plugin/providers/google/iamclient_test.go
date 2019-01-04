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

package iamclient

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"

	iam "google.golang.org/genproto/googleapis/iam/credentials/v1"
	"google.golang.org/grpc"
)

const mockServerAddress = "localhost:0"

var (
	fakeAccessToken           = "facetoken"
	fakeAccessTokenExpireTime = time.Date(2050, 12, 31, 1, 0, 0, 0, time.UTC)
)

type mockIAMServer struct {
	GenerateGoodToken bool
}

func (is *mockIAMServer) GenerateIdentityBindingAccessToken(ctx context.Context,
	in *iam.GenerateIdentityBindingAccessTokenRequest) (*iam.GenerateIdentityBindingAccessTokenResponse, error) {
	if is.GenerateGoodToken {
		et, _ := ptypes.TimestampProto(fakeAccessTokenExpireTime)
		return &iam.GenerateIdentityBindingAccessTokenResponse{
			AccessToken: fakeAccessToken,
			ExpireTime:  et,
		}, nil
	}
	return &iam.GenerateIdentityBindingAccessTokenResponse{}, fmt.Errorf("test error")
}

func (*mockIAMServer) GenerateAccessToken(context.Context, *iam.GenerateAccessTokenRequest) (*iam.GenerateAccessTokenResponse, error) {
	return nil, nil
}

// nolint:golint
func (*mockIAMServer) GenerateIdToken(context.Context, *iam.GenerateIdTokenRequest) (*iam.GenerateIdTokenResponse, error) {
	return nil, nil
}

func (*mockIAMServer) SignBlob(context.Context, *iam.SignBlobRequest) (*iam.SignBlobResponse, error) {
	return nil, nil
}

func (*mockIAMServer) SignJwt(context.Context, *iam.SignJwtRequest) (*iam.SignJwtResponse, error) {
	return nil, nil
}

func TestIAMClientPlugin(t *testing.T) {
	testHelper(t, true, false)
	testHelper(t, false, false)
	testHelper(t, true, true)
}

func testHelper(t *testing.T, createGoodToken bool, mockCertPool bool) {
	// create a local grpc server
	s := grpc.NewServer()
	defer s.Stop()
	lis, err := net.Listen("tcp", mockServerAddress)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	serv := mockIAMServer{
		GenerateGoodToken: createGoodToken,
	}

	go func() {
		iam.RegisterIAMCredentialsServer(s, &serv)
		if err := s.Serve(lis); err != nil {
			t.Fatalf("failed to serve: %v", err)
		}
	}()

	// The goroutine starting the server may not be ready, results in flakiness.
	time.Sleep(1 * time.Second)

	iamEndpoint = lis.Addr().String()
	if mockCertPool {
		tlsFlag = true
		certFunc = func() (*x509.CertPool, error) {
			return nil, fmt.Errorf("there are no certs")
		}
	} else {
		tlsFlag = false
	}
	defer func() {
		iamEndpoint = "iamcredentials.googleapis.com:443"
		tlsFlag = true
		certFunc = x509.SystemCertPool
	}()

	p := NewPlugin()
	if mockCertPool {
		if p != nil {
			t.Errorf("NewPlugin should return nil when getting certs returns error")
		}
		return
	}
	outputToken, expireTime, err := p.ExchangeToken(context.Background(), "fakeTrustDomain", "fakeInputToken")
	if createGoodToken {
		if err != nil {
			t.Fatalf("failed to call ExchangeToken: %v", err)
		}
		if outputToken != fakeAccessToken {
			t.Errorf("resp outputToken: got %+v, expected %q", outputToken, fakeAccessToken)
		}
		if expireTime != fakeAccessTokenExpireTime {
			t.Errorf("resp expireTime: got %+v, expected %q", expireTime, fakeAccessTokenExpireTime)
		}
	} else {
		if err == nil {
			t.Errorf("function ExchangeToken should return error")
		}
		strings.Compare(err.Error(), "failed to exchange token")
	}
}
