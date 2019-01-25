//  Copyright 2018 Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package echo

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/golang/sync/errgroup"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"istio.io/istio/pkg/test/application"
	"istio.io/istio/pkg/test/application/echo/proto"
)

// BatchOptions provides options to the batch processor.
type BatchOptions struct {
	Dialer  application.Dialer
	Count   int
	QPS     int
	Timeout time.Duration
	URL     string
	UDS     string
	Header  http.Header
	Message string
	CAFile  string
}

// Batch processes a Batch of requests.
type Batch struct {
	options BatchOptions
	p       protocol
}

// Run the batch and collect the results.
func (b *Batch) Run() ([]string, error) {
	g, _ := errgroup.WithContext(context.Background())
	responses := make([]string, b.options.Count)

	var throttle *time.Ticker

	if b.options.QPS > 0 {
		sleepTime := time.Second / time.Duration(b.options.QPS)
		log.Printf("Sleeping %v between requests\n", sleepTime)
		throttle = time.NewTicker(sleepTime)
	}

	for i := 0; i < b.options.Count; i++ {
		r := request{
			RequestID: i,
			URL:       b.options.URL,
			Message:   b.options.Message,
			Header:    b.options.Header,
		}
		r.RequestID = i

		if throttle != nil {
			<-throttle.C
		}

		respIndex := i
		g.Go(func() error {
			resp, err := b.p.makeRequest(&r)
			if err != nil {
				return err
			}
			responses[respIndex] = string(resp)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return responses, nil
}

// Close the batch processor.
func (b *Batch) Close() error {
	if b.p != nil {
		return b.p.Close()
	}
	return nil
}

// NewBatch creates a new batch processor with the given options.
func NewBatch(ops BatchOptions) (*Batch, error) {
	// Fill in the dialer with defaults.
	ops.Dialer = ops.Dialer.Fill()

	p, err := newProtocol(ops)
	if err != nil {
		return nil, err
	}

	b := &Batch{
		p:       p,
		options: ops,
	}

	return b, nil
}

func newProtocol(ops BatchOptions) (protocol, error) {
	var httpDialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	var wsDialContext func(network, addr string) (net.Conn, error)
	if len(ops.UDS) > 0 {
		httpDialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", ops.UDS)
		}

		wsDialContext = func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", ops.UDS)
		}
	}

	if strings.HasPrefix(ops.URL, "http://") || strings.HasPrefix(ops.URL, "https://") {
		/* #nosec */
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				DialContext: httpDialContext,
			},
			Timeout: ops.Timeout,
		}
		return &httpProtocol{
			client: client,
			do:     ops.Dialer.HTTP,
		}, nil
	} else if strings.HasPrefix(ops.URL, "grpc://") || strings.HasPrefix(ops.URL, "grpcs://") {
		secure := strings.HasPrefix(ops.URL, "grpcs://")
		var address string
		if secure {
			address = ops.URL[len("grpcs://"):]
		} else {
			address = ops.URL[len("grpc://"):]
		}

		// grpc-go sets incorrect authority header
		authority := ops.Header.Get(hostKey)

		// transport security
		security := grpc.WithInsecure()
		if secure {
			creds, err := credentials.NewClientTLSFromFile(ops.CAFile, authority)
			if err != nil {
				log.Fatalf("failed to load client certs %s %v", ops.CAFile, err)
			}
			security = grpc.WithTransportCredentials(creds)
		}

		ctx, cancel := context.WithTimeout(context.Background(), ops.Timeout)
		defer cancel()

		grpcConn, err := ops.Dialer.GRPC(ctx,
			address,
			security,
			grpc.WithAuthority(authority),
			grpc.WithBlock())
		if err != nil {
			return nil, err
		}
		client := proto.NewEchoTestServiceClient(grpcConn)
		return &grpcProtocol{
			conn:   grpcConn,
			client: client,
		}, nil
	} else if strings.HasPrefix(ops.URL, "ws://") || strings.HasPrefix(ops.URL, "wss://") {
		/* #nosec */
		dialer := &websocket.Dialer{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			NetDial:          wsDialContext,
			HandshakeTimeout: ops.Timeout,
		}
		return &websocketProtocol{
			dialer: dialer,
		}, nil
	}

	return nil, fmt.Errorf("unrecognized protocol %q", ops.URL)
}
