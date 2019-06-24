package models

import (
	"fmt"
	"strings"
	"sync"
)

type RouteGUID string

type Route struct {
	GUID     RouteGUID
	Host     string
	Path     string
	Internal bool
	VIP      string
}

func (r *Route) Hostname() string {
	return strings.ToLower(r.Host)
}

func (r *Route) GetPath() string {
	return r.Path
}

type RoutesRepo struct {
	repo           map[RouteGUID]*Route
	repoByHostname map[string]RouteGUID
	sync.Mutex
}

func NewRoutesRepo() *RoutesRepo {
	return &RoutesRepo{
		repo:           make(map[RouteGUID]*Route),
		repoByHostname: make(map[string]RouteGUID),
	}
}

func (r *RoutesRepo) Upsert(route *Route) {
	r.Lock()
	r.repo[route.GUID] = route
	r.repoByHostname[route.Host] = route.GUID
	r.Unlock()
}

func (r *RoutesRepo) Delete(guid RouteGUID) {
	r.Lock()
	delete(r.repo, guid)
	r.Unlock()
}

func (r *RoutesRepo) Sync(routes []*Route) {
	repo := make(map[RouteGUID]*Route)
	repoByHostname := make(map[string]RouteGUID)
	for _, route := range routes {
		repo[route.GUID] = route
		repoByHostname[route.Host] = route.GUID
	}
	r.Lock()
	r.repo = repo
	r.repoByHostname = repoByHostname
	r.Unlock()
}

func (r *RoutesRepo) Get(guid RouteGUID) (*Route, bool) {
	r.Lock()
	route, ok := r.repo[guid]
	r.Unlock()
	return route, ok
}

func (r *RoutesRepo) GetVIPByName(hostname string) (string, bool) {
	guid, _ := r.getGUIDByHostname(hostname)
	route, ok := r.Get(guid)
	if !ok {
		return "", ok
	}
	return route.VIP, true
}

// TODO: probably remove or clean this up, currently using for debugging
func (r *RoutesRepo) List() map[string]string {
	list := make(map[string]string)

	r.Lock()
	for k, v := range r.repo {
		list[string(k)] = v.Host
	}
	r.Unlock()

	return list
}

func (r *RoutesRepo) getGUIDByHostname(hostname string) (RouteGUID, bool) {
	r.Lock()
	guid, ok := r.repoByHostname[hostname]
	r.Unlock()
	return guid, ok
}
