package routes

import (
	"sort"
	"strings"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o fakes/routes_repo.go --fake-name RoutesRepo . routesRepo
type routesRepo interface {
	Get(guid models.RouteGUID) (*models.Route, bool)
}

//go:generate counterfeiter -o fakes/route_mappings.go --fake-name RouteMappings . routeMappings
type routeMappings interface {
	GetCalculatedWeight(rm *models.RouteMapping) int32
	List() map[string]*models.RouteMapping
}

//go:generate counterfeiter -o fakes/capi_diego.go --fake-name CapiDiego . capiDiego
type capiDiego interface {
	Get(capiProcessGUID *models.CAPIProcessGUID) *models.CAPIDiegoProcessAssociation
}

//go:generate counterfeiter -o fakes/backend_set.go --fake-name BackendSet . backendSet
type backendSet interface {
	Get(guid models.DiegoProcessGUID) *api.BackendSet
}

type Collector struct {
	logger        lager.Logger
	routesRepo    routesRepo
	routeMappings routeMappings
	capiDiego     capiDiego
	backendSet    backendSet
}

func NewCollector(logger lager.Logger, rr routesRepo, rm routeMappings, cd capiDiego, bs backendSet) *Collector {
	return &Collector{
		logger:        logger,
		routesRepo:    rr,
		routeMappings: rm,
		capiDiego:     cd,
		backendSet:    bs,
	}
}

func (c *Collector) Collect() []*api.RouteWithBackends {
	type RouteDetails struct {
		hostname        string
		backendSet      *api.BackendSet
		path            string
		capiProcessGUID string
	}

	var routesWithoutPath, routes []*api.RouteWithBackends

	for _, routeMapping := range c.routeMappings.List() {
		route, ok := c.routesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}

		if strings.HasSuffix(route.Hostname(), ".apps.internal") {
			continue
		}

		capiDiegoProcessAssociation := c.capiDiego.Get(&routeMapping.CAPIProcessGUID)
		if capiDiegoProcessAssociation == nil {
			continue
		}

		var backends []*api.Backend
		for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
			backendSet := c.backendSet.Get(models.DiegoProcessGUID(diegoProcessGUID))
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
			CapiProcessGuid: string(routeMapping.CAPIProcessGUID),
			RouteWeight:     c.routeMappings.GetCalculatedWeight(routeMapping),
		}

		if route.Path != "" {
			routes = append(routes, builtRoute)
		} else {
			routesWithoutPath = append(routesWithoutPath, builtRoute)
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
