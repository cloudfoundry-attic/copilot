package handlers

import (
	"context"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/internalroutes"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
)

type Istio struct {
	Logger                           lager.Logger
	Collector                        collector
	RoutesRepo                       routesRepoInterface
	BackendSetRepo                   backendSetRepo
	RouteMappingsRepo                routeMappingsRepoInterface
	CAPIDiegoProcessAssociationsRepo capiDiegoProcessAssociationsRepoInterface
	InternalRoutesRepo               internalRoutesRepo
}

//go:generate counterfeiter -o fakes/backend_set_repo.go --fake-name BackendSetRepo . backendSetRepo
type backendSetRepo interface {
	Get(guid models.DiegoProcessGUID) *api.BackendSet
}

//go:generate counterfeiter -o fakes/collector.go --fake-name Collector . collector
type collector interface {
	Collect() []*api.RouteWithBackends
}

//go:generate counterfeiter -o fakes/routes_repo.go --fake-name RoutesRepo . routesRepoInterface
type routesRepoInterface interface {
	Upsert(route *models.Route)
	Delete(guid models.RouteGUID)
	Sync(routes []*models.Route)
	Get(guid models.RouteGUID) (*models.Route, bool)
	List() map[string]string
}

//go:generate counterfeiter -o fakes/route_mappings_repo.go --fake-name RouteMappingsRepo . routeMappingsRepoInterface
type routeMappingsRepoInterface interface {
	GetCalculatedWeight(rm *models.RouteMapping) int32
	Map(routeMapping *models.RouteMapping)
	Unmap(routeMapping *models.RouteMapping)
	Sync(routeMappings []*models.RouteMapping)
	List() map[string]*models.RouteMapping
}

//go:generate counterfeiter -o fakes/capi_diego_process_associations_repo.go --fake-name CAPIDiegoProcessAssociationsRepo . capiDiegoProcessAssociationsRepoInterface
type capiDiegoProcessAssociationsRepoInterface interface {
	Upsert(capiDiegoProcessAssociation *models.CAPIDiegoProcessAssociation)
	Delete(capiProcessGUID *models.CAPIProcessGUID)
	Sync(capiDiegoProcessAssociations []*models.CAPIDiegoProcessAssociation)
	List() map[models.CAPIProcessGUID]*models.DiegoProcessGUIDs
	Get(capiProcessGUID *models.CAPIProcessGUID) *models.CAPIDiegoProcessAssociation
}

type internalRoutesRepo interface {
	Get() (map[internalroutes.InternalRoute][]internalroutes.Backend, error)
}

func (c *Istio) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	c.Logger.Info("istio health check...")
	return &api.HealthResponse{Healthy: true}, nil
}

func (c *Istio) Routes(context.Context, *api.RoutesRequest) (*api.RoutesResponse, error) {
	c.Logger.Info("listing istio routes...")
	routes := c.Collector.Collect()
	return &api.RoutesResponse{Routes: routes}, nil
}

func (c *Istio) InternalRoutes(context.Context, *api.InternalRoutesRequest) (*api.InternalRoutesResponse, error) {
	hostnamesToBackends, err := c.InternalRoutesRepo.Get()
	if err != nil {
		return nil, err
	}

	internalRoutesWithBackends := []*api.InternalRouteWithBackends{}
	for internalRoute, backends := range hostnamesToBackends {
		apiBackends := []*api.Backend{}
		for _, b := range backends {
			apiBackends = append(apiBackends, &api.Backend{
				Address: b.Address,
				Port:    b.Port,
			})
		}
		internalRoutesWithBackends = append(internalRoutesWithBackends, &api.InternalRouteWithBackends{
			Hostname: internalRoute.Hostname,
			Vip:      internalRoute.VIP,
			Backends: &api.BackendSet{
				Backends: apiBackends,
			},
		})
	}

	return &api.InternalRoutesResponse{
		InternalRoutes: internalRoutesWithBackends,
	}, nil
}

func (c *Istio) getAppHostPort(netInfo bbsmodels.ActualLRPNetInfo) uint32 {
	for _, port := range netInfo.Ports {
		if port.ContainerPort != models.CF_APP_SSH_PORT {
			return port.HostPort
		}
	}
	return 0
}
