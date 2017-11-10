package handlers

import (
	"context"

	bbsmodels "code.cloudfoundry.org/bbs/models"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
)

type ProcessGUID string

func (p ProcessGUID) Hostname() string {
	return string(p) + ".internal.tld"
}

type BBSClient interface {
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

type Copilot struct {
	BBSClient
	Logger lager.Logger
}

func (c *Copilot) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	return &api.HealthResponse{Healthy: true}, nil
}

func (c *Copilot) Routes(context.Context, *api.RoutesRequest) (*api.RoutesResponse, error) {
	actualLRPGroups, err := c.BBSClient.ActualLRPGroups(c.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})
	if err != nil {
		return nil, err
	}
	backends := make(map[string]*api.BackendSet)
	for _, actualGroup := range actualLRPGroups {
		instance := actualGroup.Instance
		if instance == nil {
			c.Logger.Debug("skipping-nil-instance")
			continue
		}
		id := ProcessGUID(instance.ActualLRPKey.ProcessGuid)
		hostname := id.Hostname()
		if instance.State != bbsmodels.ActualLRPStateRunning {
			c.Logger.Debug("skipping-non-running-instance", lager.Data{"process-guid": id})
			continue
		}
		if _, ok := backends[hostname]; !ok {
			backends[hostname] = &api.BackendSet{}
		}
		backends[hostname].Backends = append(backends[hostname].Backends, &api.Backend{
			Address: instance.ActualLRPNetInfo.Address,
			Port:    instance.ActualLRPNetInfo.Ports[0].HostPort,
		})
	}
	return &api.RoutesResponse{Backends: backends}, nil
}
