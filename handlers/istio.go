package handlers

import (
	"context"
	"errors"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
)

type Istio struct {
	BBSClient
	Logger                           lager.Logger
	RoutesRepo                       routesRepoInterface
	RouteMappingsRepo                routeMappingsRepoInterface
	CAPIDiegoProcessAssociationsRepo capiDiegoProcessAssociationsRepoInterface
}

func (c *Istio) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	c.Logger.Info("istio health check...")
	return &api.HealthResponse{Healthy: true}, nil
}

func (c *Istio) Routes(context.Context, *api.RoutesRequest) (*api.RoutesResponse, error) {
	c.Logger.Info("listing istio routes...")
	if c.BBSClient == nil {
		return nil, errors.New("communication with bbs is disabled")
	}

	actualLRPGroups, err := c.BBSClient.ActualLRPGroups(c.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})
	if err != nil {
		return nil, err
	}

	runningBackends := make(map[DiegoProcessGUID]*api.BackendSet)
	for _, actualGroup := range actualLRPGroups {
		instance := actualGroup.Instance
		if instance == nil {
			c.Logger.Debug("skipping-nil-instance")
			continue
		}
		diegoProcessGUID := DiegoProcessGUID(instance.ActualLRPKey.ProcessGuid)
		if instance.State != bbsmodels.ActualLRPStateRunning {
			c.Logger.Debug("skipping-non-running-instance", lager.Data{"process-guid": diegoProcessGUID})
			continue
		}
		if _, ok := runningBackends[diegoProcessGUID]; !ok {
			runningBackends[diegoProcessGUID] = &api.BackendSet{}
		}
		var appHostPort uint32
		for _, port := range instance.ActualLRPNetInfo.Ports {
			if port.ContainerPort == CF_APP_PORT {
				appHostPort = port.HostPort
			}
		}
		runningBackends[diegoProcessGUID].Backends = append(runningBackends[diegoProcessGUID].Backends, &api.Backend{
			Address: instance.ActualLRPNetInfo.Address,
			Port:    appHostPort,
		})
	}

	allBackends := make(map[string]*api.BackendSet)
	// append internal routes
	for diegoProcessGUID, backendSet := range runningBackends {
		hostname := string(diegoProcessGUID.InternalHostname())
		allBackends[hostname] = backendSet
	}

	// append external routes
	for _, routeMapping := range c.RouteMappingsRepo.List() {
		route, ok := c.RoutesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}

		for _, diegoProcessGUID := range c.CAPIDiegoProcessAssociationsRepo.List()[string(routeMapping.CAPIProcessGUID)] {
			backends, ok := runningBackends[DiegoProcessGUID(diegoProcessGUID)]
			if !ok {
				continue
			}
			if _, ok := allBackends[route.Hostname()]; !ok {
				allBackends[route.Hostname()] = &api.BackendSet{Backends: []*api.Backend{}}
			}
			allBackends[route.Hostname()].Backends = append(allBackends[route.Hostname()].Backends, backends.Backends...)
		}
	}

	return &api.RoutesResponse{Backends: allBackends}, nil
}
