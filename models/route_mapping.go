package models

import (
	"sync"
)

type CAPIProcessGUID string

type RouteMapping struct {
	RouteGUID       RouteGUID
	CAPIProcessGUID CAPIProcessGUID
	RouteWeight     uint32
}

func (r *RouteMapping) Key() string {
	return string(r.RouteGUID) + "-" + string(r.CAPIProcessGUID)
}

type RouteMappingsRepo struct {
	repo              map[string]*RouteMapping
	weightDenominator map[RouteGUID]uint32
	sync.Mutex
}

func NewRouteMappingsRepo() *RouteMappingsRepo {
	return &RouteMappingsRepo{
		repo:              make(map[string]*RouteMapping),
		weightDenominator: make(map[RouteGUID]uint32),
	}
}

func (r *RouteMappingsRepo) GetCalculatedWeight(rm *RouteMapping) int32 {
	r.Lock()
	defer r.Unlock()
	return int32((float32(rm.RouteWeight) / float32(r.weightDenominator[rm.RouteGUID])) * 100)
}

func (m *RouteMappingsRepo) Map(rm *RouteMapping) {
	m.Lock()
	m.weightDenominator[rm.RouteGUID] += rm.RouteWeight
	m.repo[rm.Key()] = rm
	m.Unlock()
}

func (m *RouteMappingsRepo) Unmap(rm *RouteMapping) {
	m.Lock()
	delete(m.repo, rm.Key())
	m.weightDenominator[rm.RouteGUID] -= rm.RouteWeight
	m.Unlock()
}

func (m *RouteMappingsRepo) Sync(routeMappings []*RouteMapping) {
	repo := make(map[string]*RouteMapping)
	weightDenominator := make(map[RouteGUID]uint32)

	for _, rm := range routeMappings {
		repo[rm.Key()] = rm
		weightDenominator[rm.RouteGUID] += rm.RouteWeight
	}

	m.Lock() // TODO: move this to the beginning for the function
	m.repo = repo
	m.weightDenominator = weightDenominator
	m.Unlock() // TODO: defer this
}

func (m *RouteMappingsRepo) List() map[string]*RouteMapping {
	list := make(map[string]*RouteMapping)

	m.Lock()
	for k, v := range m.repo {
		list[k] = v
	}
	m.Unlock()

	return list
}
