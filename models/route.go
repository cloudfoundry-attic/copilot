package models

import (
	"strings"
	"sync"
)

type RouteGUID string

type Route struct {
	GUID         RouteGUID
	Host         string
	Path         string
	Destinations []*Destination
}

type Destination struct {
	CAPIProcessGUID string
	Weight          uint32
	Port            uint32
}

func (r *Route) Hostname() string {
	return strings.ToLower(r.Host)
}

func (r *Route) GetPath() string {
	return r.Path
}

func (r *Route) GetDestinations() []*Destination {
	return r.Destinations
}

type RoutesRepo struct {
	repo map[RouteGUID]*Route
	sync.Mutex
}

func NewRoutesRepo() *RoutesRepo {
	return &RoutesRepo{
		repo: make(map[RouteGUID]*Route),
	}
}

func (r *RoutesRepo) Upsert(route *Route) {
	r.Lock()
	r.repo[route.GUID] = route
	r.Unlock()
}

func (r *RoutesRepo) Delete(guid RouteGUID) {
	r.Lock()
	delete(r.repo, guid)
	r.Unlock()
}

func (r *RoutesRepo) Sync(routes []*Route) {
	repo := make(map[RouteGUID]*Route)
	for _, route := range routes {
		repo[route.GUID] = route
	}
	r.Lock()
	r.repo = repo
	r.Unlock()
}

func (r *RoutesRepo) Get(guid RouteGUID) (*Route, bool) {
	r.Lock()
	route, ok := r.repo[guid]
	r.Unlock()
	return route, ok
}

func (r *RoutesRepo) List() map[string]*Route {
	out := make(map[string]*Route)

	r.Lock()
	for routeGUID, route := range r.repo {
		out[string(routeGUID)] = route
	}
	r.Unlock()

	return out
}
