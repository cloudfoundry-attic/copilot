package models

import (
	"sync"
)

type CAPIProcessGUID string

type RouteMapping struct {
	RouteGUID       RouteGUID
	CAPIProcessGUID CAPIProcessGUID
	RouteWeight     int32
}

func (r *RouteMapping) Key() string {
	return string(r.RouteGUID) + "-" + string(r.CAPIProcessGUID)
}

type RouteMappingsRepo struct {
	repo              map[string]*RouteMapping
	weightDenominator map[RouteGUID]int32
	sync.Mutex
}

func NewRouteMappingsRepo() *RouteMappingsRepo {
	return &RouteMappingsRepo{
		repo:              make(map[string]*RouteMapping),
		weightDenominator: make(map[RouteGUID]int32),
	}
}

func (r *RouteMappingsRepo) GetCalculatedWeight(rm *RouteMapping) int32 {
	var percent float32

	r.Lock()
	if rm.RouteWeight != 0 {
		percent = (float32(rm.RouteWeight) / float32(r.weightDenominator[rm.RouteGUID])) * 100
	} else {
		percent = 100
	}
	r.Unlock()

	return int32(percent)
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
	weightDenominator := make(map[RouteGUID]int32)

	for _, rm := range routeMappings {
		repo[rm.Key()] = rm
		weightDenominator[rm.RouteGUID] += rm.RouteWeight
	}

	m.Lock()
	m.repo = repo
	m.weightDenominator = weightDenominator
	m.Unlock()
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
