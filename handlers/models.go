package handlers

import (
	bbsmodels "code.cloudfoundry.org/bbs/models"

	"code.cloudfoundry.org/lager"
)

const CF_APP_PORT = 8080

type Process struct {
	GUID ProcessGUID
}
type ProcessGUID string
type Hostname string
type RouteGUID string

type RoutesRepo map[RouteGUID]*Route

func (r RoutesRepo) Upsert(route *Route) {
	r[route.GUID] = route
}
func (r RoutesRepo) Delete(guid RouteGUID) {
	delete(r, guid)
}
func (r RoutesRepo) Get(guid RouteGUID) (*Route, bool) {
	route, ok := r[guid]
	return route, ok
}

//go:generate counterfeiter -o fakes/routes_repo.go --fake-name RoutesRepo . routesRepoInterface
type routesRepoInterface interface {
	Upsert(route *Route)
	Delete(guid RouteGUID)
	Get(guid RouteGUID) (*Route, bool)
}

type RouteMappingsRepo map[string]*RouteMapping

func (m RouteMappingsRepo) Map(routeMapping *RouteMapping) {
	m[routeMapping.Key()] = routeMapping
}

func (m RouteMappingsRepo) Unmap(routeMapping *RouteMapping) {
	delete(m, routeMapping.Key())
}

func (m RouteMappingsRepo) List() map[string]*RouteMapping {
	return m
}

//go:generate counterfeiter -o fakes/route_mappings_repo.go --fake-name RouteMappingsRepo . routeMappingsRepoInterface
type routeMappingsRepoInterface interface {
	Map(routeMapping *RouteMapping)
	Unmap(routeMapping *RouteMapping)
	List() map[string]*RouteMapping
}

func (p ProcessGUID) Hostname() string {
	return string(p) + ".cfapps.internal"
}

type BBSClient interface {
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

type Route struct {
	GUID RouteGUID
	Host string
}

func (r *Route) Hostname() string {
	return r.Host
}

type RouteMapping struct {
	RouteGUID RouteGUID
	Process   *Process
}

func (r *RouteMapping) Key() string {
	return string(r.RouteGUID) + "-" + string(r.Process.GUID)
}
