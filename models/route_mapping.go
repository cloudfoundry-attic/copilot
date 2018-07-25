package models

import "sync"

type CAPIProcessGUID string

type RouteMapping struct {
	RouteGUID       RouteGUID
	CAPIProcessGUID CAPIProcessGUID
}

func (r *RouteMapping) Key() string {
	return string(r.RouteGUID) + "-" + string(r.CAPIProcessGUID)
}

type RouteMappingsRepo struct {
	Repo map[string]*RouteMapping
	sync.Mutex
}

func (m *RouteMappingsRepo) Map(routeMapping *RouteMapping) {
	m.Lock()
	m.Repo[routeMapping.Key()] = routeMapping
	m.Unlock()
}

func (m *RouteMappingsRepo) Unmap(routeMapping *RouteMapping) {
	m.Lock()
	delete(m.Repo, routeMapping.Key())
	m.Unlock()
}

func (m *RouteMappingsRepo) Sync(routeMappings []*RouteMapping) {
	repo := make(map[string]*RouteMapping)
	for _, routeMapping := range routeMappings {
		repo[routeMapping.Key()] = routeMapping
	}
	m.Lock()
	m.Repo = repo
	m.Unlock()
}

func (m *RouteMappingsRepo) List() map[string]*RouteMapping {
	list := make(map[string]*RouteMapping)

	m.Lock()
	for k, v := range m.Repo {
		list[k] = v
	}
	m.Unlock()

	return list
}
