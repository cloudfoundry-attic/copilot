package handlers

import (
	"context"
	"sort"
	"strings"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/internalroutes"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
)

type Istio struct {
	Logger                           lager.Logger
	RoutesRepo                       routesRepoInterface
	BackendSetRepo                   backendSetRepo
	CAPIDiegoProcessAssociationsRepo capiDiegoProcessAssociationsRepoInterface
	InternalRoutesRepo               internalRoutesRepo
}

//go:generate counterfeiter -o fakes/backend_set_repo.go --fake-name BackendSetRepo . backendSetRepo
type backendSetRepo interface {
	Get(guid models.DiegoProcessGUID) *api.BackendSet
}

//go:generate counterfeiter -o fakes/routes_repo.go --fake-name RoutesRepo . routesRepoInterface
type routesRepoInterface interface {
	Upsert(route *models.Route)
	Delete(guid models.RouteGUID)
	Sync(routes []*models.Route)
	Get(guid models.RouteGUID) (*models.Route, bool)
	List() map[string]*models.Route
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
	routes := c.collectRoutes()
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

func (c *Istio) collectRoutes() []*api.RouteWithBackends {
	type RouteDetails struct {
		hostname        string
		backendSet      *api.BackendSet
		path            string
		capiProcessGUID string
	}

	var routesWithoutPath, routes []*api.RouteWithBackends

	for _, route := range c.RoutesRepo.List() {
		if strings.HasSuffix(route.Hostname(), ".apps.internal") {
			continue
		}

		for _, destination := range route.GetDestinations() {
			capiProcessGUID := models.CAPIProcessGUID(destination.CAPIProcessGUID)
			capiDiegoProcessAssociation := c.CAPIDiegoProcessAssociationsRepo.Get(&capiProcessGUID)
			if capiDiegoProcessAssociation == nil {
				continue
			}

			var backends []*api.Backend
			for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
				backendSet := c.BackendSetRepo.Get(models.DiegoProcessGUID(diegoProcessGUID))
				if backendSet == nil {
					continue
				}

				backends = append(backends, backendSet.Backends...)
			}

			sort.SliceStable(backends, func(i, j int) bool {
				return backends[i].Address < backends[j].Address
			})

			builtRoute := &api.RouteWithBackends{
				Hostname:        route.Hostname(),
				Path:            route.Path,
				Backends:        &api.BackendSet{Backends: backends},
				CapiProcessGuid: string(capiProcessGUID),
				RouteWeight:     int32(destination.Weight),
			}

			if route.Path != "" {
				routes = append(routes, builtRoute)
			} else {
				routesWithoutPath = append(routesWithoutPath, builtRoute)
			}
		}
	}

	sort.SliceStable(routes, func(i, j int) bool {
		return len(routes[i].Path) < len(routes[j].Path)
	})

	sort.SliceStable(routesWithoutPath, func(i, j int) bool {
		return routesWithoutPath[i].CapiProcessGuid < routesWithoutPath[j].CapiProcessGuid
	})

	routes = append(routes, routesWithoutPath...)

	return routes
}
