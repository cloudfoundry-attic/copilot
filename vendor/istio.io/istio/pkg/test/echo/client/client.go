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

package client

import (
	"context"
	"io"

	"google.golang.org/grpc"

	"istio.io/istio/pkg/test/echo/common"
	"istio.io/istio/pkg/test/echo/proto"
)

var _ io.Closer = &Instance{}

// Instance is a client of an Echo server that simplifies request/response processing for Forward commands.
type Instance struct {
	conn   *grpc.ClientConn
	client proto.EchoTestServiceClient
}

// New creates a new echo client.Instance that is connected to the given server address.
func New(address string) (*Instance, error) {
	// Connect to the GRPC (command) endpoint of 'this' app.
	ctx, cancel := context.WithTimeout(context.Background(), common.ConnectionTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx,
		address,
		grpc.WithInsecure(),
		grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	client := proto.NewEchoTestServiceClient(conn)

	return &Instance{
		conn:   conn,
		client: client,
	}, nil
}

// Close the EchoClient and free any resources.
func (c *Instance) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ForwardEcho sends the given forward request and parses the response for easier processing. Only fails if the request fails.
func (c *Instance) ForwardEcho(ctx context.Context, request *proto.ForwardEchoRequest) (ParsedResponses, error) {
	// Forward a request from 'this' service to the destination service.
	resp, err := c.client.ForwardEcho(ctx, request)
	if err != nil {
		return nil, err
	}

	return parseForwardedResponse(resp), nil
}
