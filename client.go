package copilot

import (
	"crypto/tls"
	"fmt"
	"io"

	"code.cloudfoundry.org/copilot/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type IstioClient interface {
	api.IstioCopilotClient
	io.Closer
}

type istioClient struct {
	api.IstioCopilotClient
	*grpc.ClientConn
}

func NewIstioClient(serverAddress string, tlsConfig *tls.Config) (IstioClient, error) {
	conn, err := grpc.Dial(serverAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %s", err)
	}

	return &istioClient{
		IstioCopilotClient: api.NewIstioCopilotClient(conn),
		ClientConn:         conn,
	}, nil
}
