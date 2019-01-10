package routes

import (
	"sort"

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
	Get(guid models.DiegoProcessGUID) *models.BackendSet
	GetInternalBackends(guid models.DiegoProcessGUID) *models.BackendSet
}

//go:generate counterfeiter -o fakes/vip_provider.go --fake-name VIPProvider . vipProvider
type vipProvider interface {
	Get(hostname string) string
}

type Collector struct {
	logger        lager.Logger
	routesRepo    routesRepo
	routeMappings routeMappings
	capiDiego     capiDiego
	backendSet    backendSet
	vipProvider   vipProvider
}

func NewCollector(logger lager.Logger, rr routesRepo, rm routeMappings, cd capiDiego, bs backendSet, vp vipProvider) *Collector {
	return &Collector{
		logger:        logger,
		routesRepo:    rr,
		routeMappings: rm,
		capiDiego:     cd,
		backendSet:    bs,
		vipProvider:   vp,
	}
}

func (c *Collector) Collect() []*models.RouteWithBackends {
	type RouteDetails struct {
		hostname        string
		backendSet      *models.BackendSet
		path            string
		capiProcessGUID string
	}

	var routesWithoutPath, routes []*models.RouteWithBackends

	for _, routeMapping := range c.routeMappings.List() {
		route, ok := c.routesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}

		capiDiegoProcessAssociation := c.capiDiego.Get(&routeMapping.CAPIProcessGUID)
		if capiDiegoProcessAssociation == nil {
			continue
		}

		var backends []*models.Backend
		for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
			var backendSet *models.BackendSet
			if route.Internal {
				backendSet = c.backendSet.GetInternalBackends(models.DiegoProcessGUID(diegoProcessGUID))
			} else {
				backendSet = c.backendSet.Get(models.DiegoProcessGUID(diegoProcessGUID))
			}
			if backendSet == nil {
				continue
			}

			backends = append(backends, backendSet.Backends...)
		}
		if len(backends) == 0 {
			continue
		}

		sort.SliceStable(backends, func(i, j int) bool {
			return backends[i].Address < backends[j].Address
		})

		var vip string
		if route.Internal {
			vip = c.vipProvider.Get(route.Hostname())
		}

		builtRoute := &models.RouteWithBackends{
			Hostname:        route.Hostname(),
			Path:            route.Path,
			Backends:        models.BackendSet{Backends: backends},
			CapiProcessGUID: string(routeMapping.CAPIProcessGUID),
			RouteWeight:     c.routeMappings.GetCalculatedWeight(routeMapping),
			Internal:        route.Internal,
			VIP:             vip,
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
		return routesWithoutPath[i].CapiProcessGUID < routesWithoutPath[j].CapiProcessGUID
	})

	routes = append(routes, routesWithoutPath...)

	routes = fixRouteWeights(routes)

	return routes
}

func routeKey(route *models.RouteWithBackends) string {
	return route.Hostname + "/" + route.Path
}

func fixRouteWeights(routes []*models.RouteWithBackends) []*models.RouteWithBackends {
	type RouteInfo struct {
		Sum      int32
		Indicies []int
	}

	if len(routes) > 0 {
		sumPerRoute := make(map[string]RouteInfo)
		for i, route := range routes {
			key := routeKey(route)
			info := sumPerRoute[key]
			info.Sum += route.RouteWeight
			info.Indicies = append(info.Indicies, i)
			sumPerRoute[key] = info
		}

		for _, info := range sumPerRoute {
			leftover := 100 - info.Sum
			for _, index := range info.Indicies {
				if leftover == 0 {
					break
				}
				routes[index].RouteWeight += 1
				leftover -= 1
			}
		}
	}

	return routes
}
