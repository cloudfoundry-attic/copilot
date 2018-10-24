package copilot

import (
	"crypto/tls"
	"fmt"
	"io"

	"code.cloudfoundry.org/copilot/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %s", err)
	}

	return &cloudControllerClient{
		CloudControllerCopilotClient: api.NewCloudControllerCopilotClient(conn),
		ClientConn:                   conn,
	}, nil
}
