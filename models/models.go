package models

import (
	"sync"
)

const CF_APP_SSH_PORT = 2222

type RouteGUID string

type Route struct {
	GUID RouteGUID
	Host string
}

func (r *Route) Hostname() string {
	return r.Host
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

type DiegoProcessGUID string

type DiegoProcessGUIDs []DiegoProcessGUID

func DiegoProcessGUIDsFromStringSlice(diegoProcessGUIDs []string) DiegoProcessGUIDs {
	diegoGUIDs := DiegoProcessGUIDs{}
	for _, diegoGUID := range diegoProcessGUIDs {
		diegoGUIDs = append(diegoGUIDs, DiegoProcessGUID(diegoGUID))
	}
	return diegoGUIDs
}

func (p DiegoProcessGUIDs) ToStringSlice() []string {
	diegoGUIDs := []string{}
	for _, diegoGUID := range p {
		diegoGUIDs = append(diegoGUIDs, string(diegoGUID))
	}
	return diegoGUIDs
}

type CAPIDiegoProcessAssociationsRepo struct {
	Repo map[CAPIProcessGUID]*CAPIDiegoProcessAssociation
	sync.Mutex
}

type CAPIDiegoProcessAssociation struct {
	CAPIProcessGUID   CAPIProcessGUID
	DiegoProcessGUIDs DiegoProcessGUIDs
}

func (c *CAPIDiegoProcessAssociationsRepo) Upsert(capiDiegoProcessAssociation *CAPIDiegoProcessAssociation) {
	c.Lock()
	c.Repo[capiDiegoProcessAssociation.CAPIProcessGUID] = capiDiegoProcessAssociation
	c.Unlock()
}

func (c *CAPIDiegoProcessAssociationsRepo) Delete(capiProcessGUID *CAPIProcessGUID) {
	c.Lock()
	delete(c.Repo, *capiProcessGUID)
	c.Unlock()
}

func (c *CAPIDiegoProcessAssociationsRepo) Sync(capiDiegoProcessAssociations []*CAPIDiegoProcessAssociation) {
	repo := make(map[CAPIProcessGUID]*CAPIDiegoProcessAssociation)
	for _, capiDiegoProcessAssociation := range capiDiegoProcessAssociations {
		repo[capiDiegoProcessAssociation.CAPIProcessGUID] = capiDiegoProcessAssociation
	}
	c.Lock()
	c.Repo = repo
	c.Unlock()
}

func (c *CAPIDiegoProcessAssociationsRepo) List() map[CAPIProcessGUID]*DiegoProcessGUIDs {
	list := make(map[CAPIProcessGUID]*DiegoProcessGUIDs)

	c.Lock()
	for k, v := range c.Repo {
		list[k] = &v.DiegoProcessGUIDs
	}
	c.Unlock()

	return list
}

func (c *CAPIDiegoProcessAssociationsRepo) Get(capiProcessGUID *CAPIProcessGUID) *CAPIDiegoProcessAssociation {
	c.Lock()
	capiDiegoProcessAssociation, _ := c.Repo[*capiProcessGUID]
	c.Unlock()
	return capiDiegoProcessAssociation
}
