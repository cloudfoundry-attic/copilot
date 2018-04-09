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

	diegoProcessGUIDToBackendSet, err := c.retrieveDiegoProcessGUIDToBackendSet()
	if err != nil {
		return nil, err
	}

	return &api.RoutesResponse{Backends: c.hostnameToBackendSet(diegoProcessGUIDToBackendSet)}, nil
}

func (c *Istio) retrieveDiegoProcessGUIDToBackendSet() (map[DiegoProcessGUID]*api.BackendSet, error) {
	actualLRPGroups, err := c.BBSClient.ActualLRPGroups(c.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})
	if err != nil {
		return nil, err
	}

	diegoProcessGUIDToBackendSet := make(map[DiegoProcessGUID]*api.BackendSet)
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
		if _, ok := diegoProcessGUIDToBackendSet[diegoProcessGUID]; !ok {
			diegoProcessGUIDToBackendSet[diegoProcessGUID] = &api.BackendSet{}
		}
		var appHostPort uint32
		for _, port := range instance.ActualLRPNetInfo.Ports {
			if port.ContainerPort == CF_APP_PORT {
				appHostPort = port.HostPort
			}
		}
		diegoProcessGUIDToBackendSet[diegoProcessGUID].Backends = append(diegoProcessGUIDToBackendSet[diegoProcessGUID].Backends, &api.Backend{
			Address: instance.ActualLRPNetInfo.Address,
			Port:    appHostPort,
		})
	}
	return diegoProcessGUIDToBackendSet, nil
}

func (c *Istio) hostnameToBackendSet(diegoProcessGUIDToBackendSet map[DiegoProcessGUID]*api.BackendSet) map[string]*api.BackendSet {
	hostnameToBackendSet := make(map[string]*api.BackendSet)
	for _, routeMapping := range c.RouteMappingsRepo.List() {
		route, ok := c.RoutesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}

		capiDiegoProcessAssociation := c.CAPIDiegoProcessAssociationsRepo.Get(&routeMapping.CAPIProcessGUID)
		if capiDiegoProcessAssociation == nil {
			continue
		}
		for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
			backends, ok := diegoProcessGUIDToBackendSet[DiegoProcessGUID(diegoProcessGUID)]
			if !ok {
				continue
			}
			if _, ok := hostnameToBackendSet[route.Hostname()]; !ok {
				hostnameToBackendSet[route.Hostname()] = &api.BackendSet{Backends: []*api.Backend{}}
			}
			hostnameToBackendSet[route.Hostname()].Backends = append(hostnameToBackendSet[route.Hostname()].Backends, backends.Backends...)
		}
	}
	return hostnameToBackendSet
}
