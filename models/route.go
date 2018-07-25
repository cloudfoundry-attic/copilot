package models

import (
	"strings"
	"sync"
)

type RouteGUID string

type Route struct {
	GUID RouteGUID
	Host string
	Path string
}

func (r *Route) Hostname() string {
	return strings.ToLower(r.Host)
}

func (r *Route) GetPath() string {
	return r.Path
}

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

func (r *RoutesRepo) Sync(routes []*Route) {
	repo := make(map[RouteGUID]*Route)
	for _, route := range routes {
		repo[route.GUID] = route
	}
	r.Lock()
	r.Repo = repo
	r.Unlock()
}

func (r *RoutesRepo) Get(guid RouteGUID) (*Route, bool) {
	r.Lock()
	route, ok := r.Repo[guid]
	r.Unlock()
	return route, ok
}

// TODO: probably remove or clean this up, currently using for debugging
func (r *RoutesRepo) List() map[string]string {
	list := make(map[string]string)

	r.Lock()
	for k, v := range r.Repo {
		list[string(k)] = v.Host
	}
	r.Unlock()

	return list
}
