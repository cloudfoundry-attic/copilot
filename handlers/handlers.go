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

type Process struct {
	GUID ProcessGUID
}
type ProcessGUID string
type Hostname string
type RouteGUID string
type RoutesRepo map[RouteGUID]*Route
type RouteMappingsRepo map[string]*RouteMapping

func (p ProcessGUID) Hostname() Hostname {
	return Hostname(string(p) + ".cfapps.internal")
}

type BBSClient interface {
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

type Route struct {
	GUID     RouteGUID
	Hostname Hostname
}

type RouteMapping struct {
	RouteGUID RouteGUID
	Process   *Process
}

func (r *RouteMapping) Key() string {
	return string(r.RouteGUID) + "-" + string(r.Process.GUID)
}

func validateMapRouteRequest(r *api.MapRouteRequest) error {
	if r.Process == nil {
		return errors.New("Process is required")
	}
	if r.RouteGuid == "" || r.Process.Guid == "" {
		return errors.New("RouteGUID and ProcessGUID are required")
	}
	return nil
}

func validateUnmapRouteRequest(r *api.UnmapRouteRequest) error {
	if r.RouteGuid == "" || r.ProcessGuid == "" {
		return errors.New("RouteGuid and ProcessGuid are required")
	}
	return nil
}

type Copilot struct {
	BBSClient
	Logger            lager.Logger
	RoutesRepo        RoutesRepo
	RouteMappingsRepo RouteMappingsRepo
}

func (c *Copilot) MapRoute(context context.Context, request *api.MapRouteRequest) (*api.MapRouteResponse, error) {
	err := validateMapRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route Mapping %#v is invalid:\n %v", request, err)
	}
	r := &RouteMapping{
		RouteGUID: RouteGUID(request.RouteGuid),
		Process: &Process{
			GUID: ProcessGUID(request.Process.Guid),
		},
	}

	c.RouteMappingsRepo[r.Key()] = r

	return &api.MapRouteResponse{}, nil
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
	for _, routeMapping := range c.RouteMappingsRepo {
		backends, ok := runningBackends[routeMapping.Process.GUID]
		if !ok {
			continue
		}
		route, ok := c.RoutesRepo[routeMapping.RouteGUID]
		if !ok {
			continue
		}
		if _, ok := allBackends[string(route.Hostname)]; !ok {
			allBackends[string(route.Hostname)] = &api.BackendSet{Backends: []*api.Backend{}}
		}
		allBackends[string(route.Hostname)].Backends = append(allBackends[string(route.Hostname)].Backends, backends.Backends...)
	}

	return &api.RoutesResponse{Backends: allBackends}, nil
}

func (c *Copilot) UnmapRoute(context context.Context, request *api.UnmapRouteRequest) (*api.UnmapRouteResponse, error) {
	err := validateUnmapRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route Mapping %#v is invalid:\n %v", request, err)
	}
	r := &RouteMapping{RouteGUID: RouteGUID(request.RouteGuid), Process: &Process{GUID: ProcessGUID(request.ProcessGuid)}}

	delete(c.RouteMappingsRepo, r.Key())

	return &api.UnmapRouteResponse{}, nil
}
