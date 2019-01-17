package handlers

import (
	"context"
	"fmt"
	"strings"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o fakes/routes_vip_resolver_repo.go --fake-name RoutesRepoVIPResolverInterface . routesRepoVIPResolverInterface
type routesRepoVIPResolverInterface interface {
	GetVIPByName(hostname string) (string, bool)
}

type VIPResolver struct {
	Logger     lager.Logger
	RoutesRepo routesRepoVIPResolverInterface
}

func (b *VIPResolver) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	b.Logger.Info("vip resolver health check...")
	return &api.HealthResponse{Healthy: true}, nil
}

func (b *VIPResolver) GetVIPByName(ctx context.Context, request *api.GetVIPByNameRequest) (*api.GetVIPByNameResponse, error) {
	ip, ok := b.RoutesRepo.GetVIPByName(strings.TrimRight(request.Fqdn, "."))
	if !ok {
		return nil, fmt.Errorf("route doesn't exist: %s", request.Fqdn)
	}

	return &api.GetVIPByNameResponse{
		Ip: ip,
	}, nil
}
