package handlers

import (
	"context"

	bbsmodels "code.cloudfoundry.org/bbs/models"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
)

const CF_APP_PORT = 8080

type ProcessGUID string

func (p ProcessGUID) Hostname() string {
	return string(p) + ".cfapps.internal"
}

type BBSClient interface {
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

type Copilot struct {
	BBSClient
	Logger     lager.Logger
	RoutesRepo map[string]*Route
}

func (c *Copilot) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	return &api.HealthResponse{Healthy: true}, nil
}

func (c *Copilot) Routes(context.Context, *api.RoutesRequest) (*api.RoutesResponse, error) {
	actualLRPGroups, err := c.BBSClient.ActualLRPGroups(c.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})

	if err != nil {
		return nil, err
	}

	guidBackends := make(map[string]*api.BackendSet)
	for _, actualGroup := range actualLRPGroups {
		instance := actualGroup.Instance
		if instance == nil {
			c.Logger.Debug("skipping-nil-instance")
			continue
		}
		processGuid := instance.ActualLRPKey.ProcessGuid
		if instance.State != bbsmodels.ActualLRPStateRunning {
			c.Logger.Debug("skipping-non-running-instance", lager.Data{"process-guid": processGuid})
			continue
		}
		if _, ok := guidBackends[processGuid]; !ok {
			guidBackends[processGuid] = &api.BackendSet{}
		}
		var appHostPort uint32
		for _, port := range instance.ActualLRPNetInfo.Ports {
			if port.ContainerPort == CF_APP_PORT {
				appHostPort = port.HostPort
			}
		}
		guidBackends[processGuid].Backends = append(guidBackends[processGuid].Backends, &api.Backend{
			Address: instance.ActualLRPNetInfo.Address,
			Port:    appHostPort,
		})
	}

	backends := make(map[string]*api.BackendSet)
	for guid, guidbackend := range guidBackends {
		id := ProcessGUID(guid)
		hostname := id.Hostname()
		backends[hostname] = guidbackend
	}

	for _, route := range c.RoutesRepo {
		if val, ok := guidBackends[route.ProcessGuid]; ok {
			if _, ok := backends[route.Hostname]; !ok {
				backends[route.Hostname] = &api.BackendSet{Backends: []*api.Backend{}}
			}
			backends[route.Hostname].Backends = append(backends[route.Hostname].Backends, val.Backends...)
		}
	}

	return &api.RoutesResponse{Backends: backends}, nil
}

type Route struct {
	Hostname    string
	ProcessGuid string
}

func (r *Route) Key() string {
	return r.Hostname + "-" + r.ProcessGuid
}

func (c *Copilot) AddRoute(context context.Context, request *api.AddRequest) (*api.AddResponse, error) {
	r := &Route{Hostname: request.Hostname, ProcessGuid: request.ProcessGuid}
	c.RoutesRepo[r.Key()] = &Route{Hostname: request.Hostname, ProcessGuid: request.ProcessGuid}
	return &api.AddResponse{Success: true}, nil
}
