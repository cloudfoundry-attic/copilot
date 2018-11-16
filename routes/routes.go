package routes

import (
	"sort"
	"strings"

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

		internal := route.Internal || strings.HasSuffix(route.Hostname(), ".apps.internal")

		var backends []*models.Backend
		for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
			var backendSet *models.BackendSet
			if internal {
				backendSet = c.backendSet.GetInternalBackends(models.DiegoProcessGUID(diegoProcessGUID))
			} else {
				backendSet = c.backendSet.Get(models.DiegoProcessGUID(diegoProcessGUID))
			}
			if backendSet == nil {
				continue
			}

			backends = append(backends, backendSet.Backends...)
		}

		sort.SliceStable(backends, func(i, j int) bool {
			return backends[i].Address < backends[j].Address
		})

		var vip string
		if internal {
			vip = c.vipProvider.Get(route.Hostname())
		}

		builtRoute := &models.RouteWithBackends{
			Hostname:        route.Hostname(),
			Path:            route.Path,
			Backends:        models.BackendSet{Backends: backends},
			CapiProcessGUID: string(routeMapping.CAPIProcessGUID),
			RouteWeight:     c.routeMappings.GetCalculatedWeight(routeMapping),
			Internal:        internal,
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

	return routes
}
