package handlers

import (
	"context"
	"errors"
	"strings"

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
	VIPProvider                      vipProvider
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

//go:generate counterfeiter -o fakes/vip_provider.go --fake-name VIPProvider . vipProvider
type vipProvider interface {
	Get(hostname string) string
}

func (c *Istio) InternalRoutes(context.Context, *api.InternalRoutesRequest) (*api.InternalRoutesResponse, error) {
	lrpNetInfosMap, err := c.retrieveActualLRPNetInfos()
	if err != nil {
		panic(err)
	}

	hostnamesToBackends := map[string][]*api.Backend{}

	for _, routeMapping := range c.RouteMappingsRepo.List() {
		route, ok := c.RoutesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}

		hostname := route.Hostname()
		if !strings.HasSuffix(hostname, ".apps.internal") {
			continue
		}

		capiDiegoProcessAssociation := c.CAPIDiegoProcessAssociationsRepo.Get(&routeMapping.CAPIProcessGUID)
		if capiDiegoProcessAssociation == nil {
			continue
		}

		allBackendsForThisRouteMapping := []*api.Backend{}
		for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
			for _, lrpNetInfo := range lrpNetInfosMap[diegoProcessGUID] {
				appContainerPort := c.getAppContainerPort(lrpNetInfo)
				if appContainerPort == 0 {
					continue
				}
				allBackendsForThisRouteMapping = append(allBackendsForThisRouteMapping, &api.Backend{
					Address: lrpNetInfo.InstanceAddress,
					Port:    appContainerPort,
				})
			}
		}

		hostnamesToBackends[hostname] = append(hostnamesToBackends[hostname], allBackendsForThisRouteMapping...)
	}

	internalRoutesWithBackends := []*api.InternalRouteWithBackends{}
	for hostname, backends := range hostnamesToBackends {
		internalRoutesWithBackends = append(internalRoutesWithBackends, &api.InternalRouteWithBackends{
			Hostname: hostname,
			Vip:      c.VIPProvider.Get(hostname),
			Backends: &api.BackendSet{
				Backends: backends,
			},
		})
	}

	return &api.InternalRoutesResponse{
		InternalRoutes: internalRoutesWithBackends,
	}, nil
}

func (c *Istio) retrieveActualLRPNetInfos() (map[DiegoProcessGUID][]bbsmodels.ActualLRPNetInfo, error) {
	actualLRPGroups, err := c.BBSClient.ActualLRPGroups(c.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})
	if err != nil {
		return nil, err
	}
	actualLRPNetInfos := make(map[DiegoProcessGUID][]bbsmodels.ActualLRPNetInfo)
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
		netInfos := actualLRPNetInfos[diegoProcessGUID]
		netInfos = append(netInfos, instance.ActualLRPNetInfo)
		actualLRPNetInfos[diegoProcessGUID] = netInfos
	}
	return actualLRPNetInfos, nil
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
		appHostPort := c.getAppHostPort(instance.ActualLRPNetInfo)
		if appHostPort == 0 {
			continue
		}

		if _, ok := diegoProcessGUIDToBackendSet[diegoProcessGUID]; !ok {
			diegoProcessGUIDToBackendSet[diegoProcessGUID] = &api.BackendSet{}
		}
		diegoProcessGUIDToBackendSet[diegoProcessGUID].Backends = append(diegoProcessGUIDToBackendSet[diegoProcessGUID].Backends, &api.Backend{
			Address: instance.ActualLRPNetInfo.Address,
			Port:    appHostPort,
		})
	}
	return diegoProcessGUIDToBackendSet, nil
}

func (c *Istio) getAppContainerPort(netInfo bbsmodels.ActualLRPNetInfo) uint32 {
	for _, port := range netInfo.Ports {
		if port.ContainerPort != CF_APP_SSH_PORT {
			return port.ContainerPort
		}
	}

	return 0
}

func (c *Istio) getAppHostPort(netInfo bbsmodels.ActualLRPNetInfo) uint32 {
	for _, port := range netInfo.Ports {
		if port.ContainerPort != CF_APP_SSH_PORT {
			return port.HostPort
		}
	}
	return 0
}

func (c *Istio) hostnameToBackendSet(diegoProcessGUIDToBackendSet map[DiegoProcessGUID]*api.BackendSet) map[string]*api.BackendSet {
	hostnameToBackendSet := make(map[string]*api.BackendSet)
	for _, routeMapping := range c.RouteMappingsRepo.List() {
		route, ok := c.RoutesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}
		if strings.HasSuffix(route.Hostname(), ".apps.internal") {
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
