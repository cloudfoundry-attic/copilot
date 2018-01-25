package handlers

import (
	"sync"

	bbsmodels "code.cloudfoundry.org/bbs/models"

	"code.cloudfoundry.org/lager"
)

const CF_APP_PORT = 8080

type CAPIProcess struct {
	GUID             string
	DiegoProcessGUID DiegoProcessGUID
}

type DiegoProcessGUID string
type Hostname string
type RouteGUID string

type RoutesRepo struct {
	Repo map[RouteGUID]*Route
	sync.Mutex
}

func (r *RoutesRepo) Upsert(route *Route) {
	r.Lock()
	r.Repo[route.GUID] = route
	r.Unlock()
}

func (r *RoutesRepo) Delete(guid RouteGUID) {
	r.Lock()
	delete(r.Repo, guid)
	r.Unlock()
}

func (r *RoutesRepo) Get(guid RouteGUID) (*Route, bool) {
	r.Lock()
	route, ok := r.Repo[guid]
	r.Unlock()
	return route, ok
}

//go:generate counterfeiter -o fakes/routes_repo.go --fake-name RoutesRepo . routesRepoInterface
type routesRepoInterface interface {
	Upsert(route *Route)
	Delete(guid RouteGUID)
	Get(guid RouteGUID) (*Route, bool)
}

type RouteMappingsRepo struct {
	Repo map[string]RouteMapping
	sync.Mutex
}

func (m *RouteMappingsRepo) Map(routeMapping RouteMapping) {
	m.Lock()
	m.Repo[routeMapping.Key()] = routeMapping
	m.Unlock()
}

func (m *RouteMappingsRepo) Unmap(routeMapping RouteMapping) {
	m.Lock()
	delete(m.Repo, routeMapping.Key())
	m.Unlock()
}

func (m *RouteMappingsRepo) List() map[string]RouteMapping {
	list := make(map[string]RouteMapping)

	m.Lock()
	for k, v := range m.Repo {
		list[k] = v
	}
	m.Unlock()

	return list
}

//go:generate counterfeiter -o fakes/route_mappings_repo.go --fake-name RouteMappingsRepo . routeMappingsRepoInterface
type routeMappingsRepoInterface interface {
	Map(routeMapping RouteMapping)
	Unmap(routeMapping RouteMapping)
	List() map[string]RouteMapping
}

func (p DiegoProcessGUID) Hostname() string {
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
	RouteGUID   RouteGUID
	CAPIProcess *CAPIProcess
}

func (r *RouteMapping) Key() string {
	return string(r.RouteGUID) + "-" + string(r.CAPIProcess.GUID)
}
