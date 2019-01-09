package handlers

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o fakes/routes_bosh_dns_adapter_repo.go --fake-name RoutesRepoBoshDNSAdapterInterface . routesRepoBoshDNSAdapterInterface
type routesRepoBoshDNSAdapterInterface interface {
	GetVIPByName(hostname string) (string, bool)
}

type BoshDNSAdapter struct {
	Logger     lager.Logger
	RoutesRepo routesRepoBoshDNSAdapterInterface
}

func (b *BoshDNSAdapter) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	b.Logger.Info("bosh dns adapter health check...")
	return &api.HealthResponse{Healthy: true}, nil
}

func (b *BoshDNSAdapter) GetVIPByName(ctx context.Context, request *api.GetVIPByNameRequest) (*api.GetVIPByNameResponse, error) {
	ip, ok := b.RoutesRepo.GetVIPByName(request.Fqdn)
	if !ok {
		return nil, fmt.Errorf("route doesn't exist: %s", request.Fqdn)
	}

	return &api.GetVIPByNameResponse{
		Ip: ip,
	}, nil
}
