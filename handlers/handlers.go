package handlers

import (
	"context"
	"errors"

	bbsmodels "code.cloudfoundry.org/bbs/models"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const CF_APP_PORT = 8080

type ProcessGUID string

type Hostname string

func (p ProcessGUID) Hostname() Hostname {
	return Hostname(string(p) + ".cfapps.internal")
}

type BBSClient interface {
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

type RouteMapping struct {
	Hostname    Hostname
	ProcessGUID ProcessGUID
}

func (r *RouteMapping) Key() string {
	return string(r.Hostname) + "-" + string(r.ProcessGUID)
}

func (r *RouteMapping) validate() error {
	if r.Hostname == "" || r.ProcessGUID == "" {
		return errors.New("Hostname and ProcessGUID are required")
	}
	return nil
}

func (c *Copilot) AddRoute(context context.Context, request *api.AddRouteRequest) (*api.AddRouteResponse, error) {
	r := &RouteMapping{Hostname: Hostname(request.Hostname), ProcessGUID: ProcessGUID(request.ProcessGuid)}
	err := r.validate()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route Mapping %#v is invalid:\n %v", r, err)
	}

	c.RoutesRepo[r.Key()] = r

	return &api.AddRouteResponse{}, nil
}

type Copilot struct {
	BBSClient
	Logger     lager.Logger
	RoutesRepo map[string]*RouteMapping
}

func (c *Copilot) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	return &api.HealthResponse{Healthy: true}, nil
}

func (c *Copilot) Routes(context.Context, *api.RoutesRequest) (*api.RoutesResponse, error) {
	actualLRPGroups, err := c.BBSClient.ActualLRPGroups(c.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})

	if err != nil {
		return nil, err
	}

	runningBackends := make(map[ProcessGUID]*api.BackendSet)
	for _, actualGroup := range actualLRPGroups {
		instance := actualGroup.Instance
		if instance == nil {
			c.Logger.Debug("skipping-nil-instance")
			continue
		}
		processGUID := ProcessGUID(instance.ActualLRPKey.ProcessGuid)
		if instance.State != bbsmodels.ActualLRPStateRunning {
			c.Logger.Debug("skipping-non-running-instance", lager.Data{"process-guid": processGUID})
			continue
		}
		if _, ok := runningBackends[processGUID]; !ok {
			runningBackends[processGUID] = &api.BackendSet{}
		}
		var appHostPort uint32
		for _, port := range instance.ActualLRPNetInfo.Ports {
			if port.ContainerPort == CF_APP_PORT {
				appHostPort = port.HostPort
			}
		}
		runningBackends[processGUID].Backends = append(runningBackends[processGUID].Backends, &api.Backend{
			Address: instance.ActualLRPNetInfo.Address,
			Port:    appHostPort,
		})
	}

	allBackends := make(map[string]*api.BackendSet)
	// append internal routes
	for processGUID, backendSet := range runningBackends {
		hostname := string(processGUID.Hostname())
		allBackends[hostname] = backendSet
	}

	// append external routes
	for _, routeMapping := range c.RoutesRepo {
		if val, ok := runningBackends[routeMapping.ProcessGUID]; ok {
			if _, ok := allBackends[string(routeMapping.Hostname)]; !ok {
				allBackends[string(routeMapping.Hostname)] = &api.BackendSet{Backends: []*api.Backend{}}
			}
			allBackends[string(routeMapping.Hostname)].Backends = append(allBackends[string(routeMapping.Hostname)].Backends, val.Backends...)
		}
	}

	return &api.RoutesResponse{Backends: allBackends}, nil
}

func (c *Copilot) DeleteRoute(context context.Context, request *api.DeleteRouteRequest) (*api.DeleteRouteResponse, error){
	r := &RouteMapping{Hostname: Hostname(request.Hostname), ProcessGUID: ProcessGUID(request.ProcessGuid)}
	err := r.validate()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route Mapping %#v is invalid:\n %v", r, err)
	}

	delete(c.RoutesRepo, r.Key())

	return &api.DeleteRouteResponse{}, nil
}
