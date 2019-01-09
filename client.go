package copilot

import (
	"crypto/tls"
	"fmt"
	"io"

	"code.cloudfoundry.org/copilot/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
)

type CloudControllerClient interface {
	api.CloudControllerCopilotClient
	io.Closer
}

type cloudControllerClient struct {
	api.CloudControllerCopilotClient
	*grpc.ClientConn
}

func NewCloudControllerClient(serverAddress string, tlsConfig *tls.Config) (CloudControllerClient, error) {
	conn, err := grpc.Dial(serverAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %s", err)
	}

	return &cloudControllerClient{
		CloudControllerCopilotClient: api.NewCloudControllerCopilotClient(conn),
		ClientConn:                   conn,
	}, nil
}

type VIPResolverCopilotClient interface {
	api.VIPResolverCopilotClient
	io.Closer
}

type vipResolverCopilotClient struct {
	api.VIPResolverCopilotClient
	*grpc.ClientConn
}

func NewVIPResolverCopilotClient(serverAddress string, tlsConfig *tls.Config) (VIPResolverCopilotClient, error) {
	conn, err := grpc.Dial(serverAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %s", err)
	}

	return &vipResolverCopilotClient{
		VIPResolverCopilotClient: api.NewVIPResolverCopilotClient(conn),
		ClientConn:               conn,
	}, nil
}
