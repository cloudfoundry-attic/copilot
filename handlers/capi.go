package handlers

import (
	"context"
	"errors"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CAPI struct {
	Logger            lager.Logger
	RoutesRepo        routesRepoInterface
	RouteMappingsRepo routeMappingsRepoInterface
}

func (c *CAPI) DeleteRoute(context context.Context, request *api.DeleteRouteRequest) (*api.DeleteRouteResponse, error) {
	err := validateDeleteRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", err)
	}
	c.RoutesRepo.Delete(RouteGUID(request.Guid))
	return &api.DeleteRouteResponse{}, nil
}

func (c *CAPI) UpsertRoute(context context.Context, request *api.UpsertRouteRequest) (*api.UpsertRouteResponse, error) {
	err := validateUpsertRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route %#v is invalid:\n %v", request, err)
	}
	route := Route{
		GUID: RouteGUID(request.Guid),
		Host: request.Host,
	}
	c.RoutesRepo.Upsert(&route)
	return &api.UpsertRouteResponse{}, nil
}

func (c *CAPI) MapRoute(context context.Context, request *api.MapRouteRequest) (*api.MapRouteResponse, error) {
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

	c.RouteMappingsRepo.Map(r)

	return &api.MapRouteResponse{}, nil
}

func (c *CAPI) UnmapRoute(context context.Context, request *api.UnmapRouteRequest) (*api.UnmapRouteResponse, error) {
	err := validateUnmapRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route Mapping %#v is invalid:\n %v", request, err)
	}
	r := &RouteMapping{RouteGUID: RouteGUID(request.RouteGuid), Process: &Process{GUID: ProcessGUID(request.ProcessGuid)}}

	c.RouteMappingsRepo.Unmap(r)

	return &api.UnmapRouteResponse{}, nil
}

func validateUpsertRouteRequest(r *api.UpsertRouteRequest) error {
	if r.Guid == "" || r.Host == "" {
		return errors.New("route Guid and Host are required")
	}
	return nil
}

func validateDeleteRouteRequest(r *api.DeleteRouteRequest) error {
	if r.Guid == "" {
		return errors.New("route Guid is required")
	}
	return nil
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
