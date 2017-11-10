package copilot

import (
	"crypto/tls"
	"fmt"
	"io"

	"code.cloudfoundry.org/copilot/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Client interface {
	api.CopilotClient
	io.Closer
}

type client struct {
	api.CopilotClient
	*grpc.ClientConn
}

func NewClient(serverAddress string, tlsConfig *tls.Config) (Client, error) {
	conn, err := grpc.Dial(serverAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %s", err)
	}

	return &client{
		CopilotClient: api.NewCopilotClient(conn),
		ClientConn:    conn,
	}, nil
}
