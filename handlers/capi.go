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

func (c *CAPI) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	c.Logger.Info("capi health check...")
	return &api.HealthResponse{Healthy: true}, nil
}

// TODO: probably remove or clean this up, currently using for debugging
func (c *CAPI) ListCfRoutes(context.Context, *api.ListCfRoutesRequest) (*api.ListCfRoutesResponse, error) {
	c.Logger.Info("listing cf routes...")
	return &api.ListCfRoutesResponse{Routes: c.RoutesRepo.List()}, nil
}

// TODO: probably remove or clean this up, currently using for debugging
func (c *CAPI) ListCfRouteMappings(context.Context, *api.ListCfRouteMappingsRequest) (*api.ListCfRouteMappingsResponse, error) {
	c.Logger.Info("listing cf route mappings...")
	routeMappings := c.RouteMappingsRepo.List()
	apiRoutMappings := make(map[string]*api.RouteMapping)
	for k, v := range routeMappings {
		apiRoutMappings[k] = &api.RouteMapping{
			CapiProcess: &api.CapiProcess{
				Guid:             string(v.CAPIProcess.GUID),
				DiegoProcessGuid: string(v.CAPIProcess.DiegoProcessGUID),
			},
			RouteGuid: string(v.RouteGUID),
		}
	}
	return &api.ListCfRouteMappingsResponse{RouteMappings: apiRoutMappings}, nil
}

func (c *CAPI) UpsertRoute(context context.Context, request *api.UpsertRouteRequest) (*api.UpsertRouteResponse, error) {
	c.Logger.Info("upserting route...")
	err := validateUpsertRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route %#v is invalid:\n %v", request, err)
	}

	route := &Route{
		GUID: RouteGUID(request.Route.Guid),
		Host: request.Route.Host,
	}

	c.RoutesRepo.Upsert(route)
	return &api.UpsertRouteResponse{}, nil
}

func (c *CAPI) DeleteRoute(context context.Context, request *api.DeleteRouteRequest) (*api.DeleteRouteResponse, error) {
	c.Logger.Info("deleting route...")
	err := validateDeleteRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", err)
	}
	c.RoutesRepo.Delete(RouteGUID(request.Guid))
	return &api.DeleteRouteResponse{}, nil
}

func (c *CAPI) MapRoute(context context.Context, request *api.MapRouteRequest) (*api.MapRouteResponse, error) {
	c.Logger.Info("mapping route...")
	err := validateMapRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route Mapping %#v is invalid:\n %v", request, err)
	}
	r := RouteMapping{
		RouteGUID: RouteGUID(request.RouteMapping.RouteGuid),
		CAPIProcess: &CAPIProcess{
			GUID:             CAPIProcessGUID(request.RouteMapping.CapiProcess.Guid),
			DiegoProcessGUID: DiegoProcessGUID(request.RouteMapping.CapiProcess.DiegoProcessGuid),
		},
	}

	c.RouteMappingsRepo.Map(r)

	return &api.MapRouteResponse{}, nil
}

func (c *CAPI) UnmapRoute(context context.Context, request *api.UnmapRouteRequest) (*api.UnmapRouteResponse, error) {
	c.Logger.Info("unmapping route...")
	err := validateUnmapRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route Mapping %#v is invalid:\n %v", request, err)
	}

	r := RouteMapping{RouteGUID: RouteGUID(request.RouteGuid), CAPIProcess: &CAPIProcess{GUID: CAPIProcessGUID(request.CapiProcessGuid)}}

	c.RouteMappingsRepo.Unmap(r)

	return &api.UnmapRouteResponse{}, nil
}

func validateUpsertRouteRequest(r *api.UpsertRouteRequest) error {
	route := r.Route
	if route == nil {
		return errors.New("route is required")
	}
	if route.Guid == "" || route.Host == "" {
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
	rm := r.RouteMapping
	if rm == nil {
		return errors.New("RouteMapping is required")
	}
	if rm.CapiProcess == nil {
		return errors.New("CapiProcess is required")
	}
	if rm.RouteGuid == "" || rm.CapiProcess.Guid == "" {
		return errors.New("RouteGUID and CapiProcessGUID are required")
	}
	return nil
}

func validateUnmapRouteRequest(r *api.UnmapRouteRequest) error {
	if r.RouteGuid == "" || r.CapiProcessGuid == "" {
		return errors.New("RouteGuid and CapiProcessGuid are required")
	}
	return nil
}
