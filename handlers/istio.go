package handlers

import (
	"context"
	"errors"
	"strings"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/internalroutes"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
)

type Istio struct {
	BBSClient
	Logger                           lager.Logger
	RoutesRepo                       routesRepoInterface
	RouteMappingsRepo                routeMappingsRepoInterface
	CAPIDiegoProcessAssociationsRepo capiDiegoProcessAssociationsRepoInterface
	InternalRoutesRepo               internalRoutesRepo
}

type BBSClient interface {
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
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
	if c.BBSClient == nil {
		return nil, errors.New("communication with bbs is disabled")
	}

	diegoProcessGUIDToBackendSet, err := c.retrieveDiegoProcessGUIDToBackendSet()
	if err != nil {
		return nil, err
	}

	return &api.RoutesResponse{Routes: c.hostnameToBackendSet(diegoProcessGUIDToBackendSet)}, nil
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

func (c *Istio) retrieveDiegoProcessGUIDToBackendSet() (map[models.DiegoProcessGUID]*api.BackendSet, error) {
	actualLRPGroups, err := c.BBSClient.ActualLRPGroups(c.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})
	if err != nil {
		return nil, err
	}

	diegoProcessGUIDToBackendSet := make(map[models.DiegoProcessGUID]*api.BackendSet)
	for _, actualGroup := range actualLRPGroups {
		instance := actualGroup.Instance
		if instance == nil {
			c.Logger.Debug("skipping-nil-instance")
			continue
		}
		diegoProcessGUID := models.DiegoProcessGUID(instance.ActualLRPKey.ProcessGuid)
		if instance.State != bbsmodels.ActualLRPStateRunning {
			c.Logger.Debug("skipping-non-running-instance", lager.Data{"process-guid": diegoProcessGUID})
			continue
		}
		appHostPort := c.getAppHostPort(instance.ActualLRPNetInfo)
		if appHostPort == 0 {
			continue
		}

		if _, ok := diegoProcessGUIDToBackendSet[diegoProcessGUID]; !ok {
			diegoProcessGUIDToBackendSet[diegoProcessGUID] = &api.BackendSet{}
		}
		diegoProcessGUIDToBackendSet[diegoProcessGUID].Backends = append(diegoProcessGUIDToBackendSet[diegoProcessGUID].Backends, &api.Backend{
			Address: instance.ActualLRPNetInfo.Address,
			Port:    appHostPort,
		})
	}
	return diegoProcessGUIDToBackendSet, nil
}

func (c *Istio) getAppHostPort(netInfo bbsmodels.ActualLRPNetInfo) uint32 {
	for _, port := range netInfo.Ports {
		if port.ContainerPort != models.CF_APP_SSH_PORT {
			return port.HostPort
		}
	}
	return 0
}

func (c *Istio) hostnameToBackendSet(diegoProcessGUIDToBackendSet map[models.DiegoProcessGUID]*api.BackendSet) []*api.RouteWithBackends {
	var routes []*api.RouteWithBackends
	hostnameToBackendSet := make(map[string]*api.BackendSet)
	for _, routeMapping := range c.RouteMappingsRepo.List() {
		route, ok := c.RoutesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}
		if strings.HasSuffix(route.Hostname(), ".apps.internal") {
			continue
		}
		capiDiegoProcessAssociation := c.CAPIDiegoProcessAssociationsRepo.Get(&routeMapping.CAPIProcessGUID)
		if capiDiegoProcessAssociation == nil {
			continue
		}
		for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
			backends, ok := diegoProcessGUIDToBackendSet[models.DiegoProcessGUID(diegoProcessGUID)]
			if !ok {
				continue
			}

			if _, ok := hostnameToBackendSet[route.Hostname()]; !ok {
				hostnameToBackendSet[route.Hostname()] = &api.BackendSet{Backends: []*api.Backend{}}
			}
			hostnameToBackendSet[route.Hostname()].Backends = append(hostnameToBackendSet[route.Hostname()].Backends, backends.Backends...)
		}
	}

	for hostname, backends := range hostnameToBackendSet {
		routes = append(routes, &api.RouteWithBackends{
			Hostname: hostname,
			Backends: backends,
		})
	}
	return routes
}
