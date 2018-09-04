package handlers

import (
	"context"
	"errors"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CAPI struct {
	Logger                           lager.Logger
	RoutesRepo                       routesRepoInterface
	CAPIDiegoProcessAssociationsRepo capiDiegoProcessAssociationsRepoInterface
}

func (c *CAPI) Health(context.Context, *api.HealthRequest) (*api.HealthResponse, error) {
	c.Logger.Info("capi health check...")
	return &api.HealthResponse{Healthy: true}, nil
}

func (c *CAPI) ListCfRoutes(context.Context, *api.ListCfRoutesRequest) (*api.ListCfRoutesResponse, error) {
	c.Logger.Info("listing cf routes...")
	routes := make(map[string]*api.Route)
	for routeGUID, route := range c.RoutesRepo.List() {
		destinations := make([]*api.Destination, len(route.Destinations))
		for i, d := range route.Destinations {
			destinations[i] = &api.Destination{CapiProcessGuid: d.CAPIProcessGUID, Weight: d.Weight, Port: d.Port}
		}
		routes[routeGUID] = &api.Route{
			Guid:         string(routeGUID),
			Host:         route.Hostname(),
			Path:         route.GetPath(),
			Destinations: destinations,
		}
	}
	return &api.ListCfRoutesResponse{Routes: routes}, nil
}

// TODO: probably remove or test these eventually, currently using for debugging
func (c *CAPI) ListCapiDiegoProcessAssociations(context.Context, *api.ListCapiDiegoProcessAssociationsRequest) (*api.ListCapiDiegoProcessAssociationsResponse, error) {
	c.Logger.Info("listing capi/diego process associations...")

	response := &api.ListCapiDiegoProcessAssociationsResponse{
		CapiDiegoProcessAssociations: make(map[string]*api.DiegoProcessGuids),
	}
	for capiProcessGUID, diegoProcessGUIDs := range c.CAPIDiegoProcessAssociationsRepo.List() {
		response.CapiDiegoProcessAssociations[string(capiProcessGUID)] = &api.DiegoProcessGuids{DiegoProcessGuids: diegoProcessGUIDs.ToStringSlice()}
	}
	return response, nil
}

func (c *CAPI) UpsertRoute(context context.Context, request *api.UpsertRouteRequest) (*api.UpsertRouteResponse, error) {
	c.Logger.Info("upserting route...")
	err := validateUpsertRouteRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Route %#v is invalid:\n %v", request, err)
	}

	destinations := make([]*models.Destination, len(request.Route.Destinations))
	for i, d := range request.Route.Destinations {
		destinations[i] = &models.Destination{CAPIProcessGUID: d.CapiProcessGuid, Weight: d.Weight, Port: d.Port}
	}

	route := &models.Route{
		GUID:         models.RouteGUID(request.Route.GetGuid()),
		Host:         request.Route.GetHost(),
		Path:         request.Route.GetPath(),
		Destinations: destinations,
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
	c.RoutesRepo.Delete(models.RouteGUID(request.Guid))
	return &api.DeleteRouteResponse{}, nil
}

func (c *CAPI) UpsertCapiDiegoProcessAssociation(context context.Context, request *api.UpsertCapiDiegoProcessAssociationRequest) (*api.UpsertCapiDiegoProcessAssociationResponse, error) {
	c.Logger.Info("upserting capi/diego process association...")
	err := validateUpsertCAPIDiegoProcessAssociationRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Capi/Diego Process Association %#v is invalid:\n %v", request, err)
	}
	association := &models.CAPIDiegoProcessAssociation{
		CAPIProcessGUID:   models.CAPIProcessGUID(request.CapiDiegoProcessAssociation.CapiProcessGuid),
		DiegoProcessGUIDs: models.DiegoProcessGUIDsFromStringSlice(request.CapiDiegoProcessAssociation.DiegoProcessGuids),
	}
	c.CAPIDiegoProcessAssociationsRepo.Upsert(association)
	return &api.UpsertCapiDiegoProcessAssociationResponse{}, nil
}

func (c *CAPI) DeleteCapiDiegoProcessAssociation(context context.Context, request *api.DeleteCapiDiegoProcessAssociationRequest) (*api.DeleteCapiDiegoProcessAssociationResponse, error) {
	c.Logger.Info("deleting capi/diego process association...")
	err := validateDeleteCAPIDiegoProcessAssociationRequest(request)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", err)
	}

	cpg := models.CAPIProcessGUID(request.CapiProcessGuid)
	c.CAPIDiegoProcessAssociationsRepo.Delete(&cpg)

	return &api.DeleteCapiDiegoProcessAssociationResponse{}, nil
}

func (c *CAPI) BulkSync(context context.Context, request *api.BulkSyncRequest) (*api.BulkSyncResponse, error) {
	c.Logger.Info("bulk sync...")

	routes := make([]*models.Route, len(request.Routes))

	for i, route := range request.Routes {
		destinations := make([]*models.Destination, len(route.Destinations))
		for i, d := range route.Destinations {
			destinations[i] = &models.Destination{CAPIProcessGUID: d.CapiProcessGuid, Weight: d.Weight, Port: d.Port}
		}

		routes[i] = &models.Route{
			GUID:         models.RouteGUID(route.GetGuid()),
			Host:         route.GetHost(),
			Path:         route.GetPath(),
			Destinations: destinations,
		}
	}

	cdpas := make([]*models.CAPIDiegoProcessAssociation, len(request.CapiDiegoProcessAssociations))

	for i, cdpa := range request.CapiDiegoProcessAssociations {
		diegoProcessGuids := make([]models.DiegoProcessGUID, len(cdpa.DiegoProcessGuids))
		for j, diegoProcessGuid := range cdpa.DiegoProcessGuids {
			diegoProcessGuids[j] = models.DiegoProcessGUID(diegoProcessGuid)
		}
		cdpas[i] = &models.CAPIDiegoProcessAssociation{
			CAPIProcessGUID:   models.CAPIProcessGUID(cdpa.CapiProcessGuid),
			DiegoProcessGUIDs: diegoProcessGuids,
		}
	}

	c.RoutesRepo.Sync(routes)
	c.CAPIDiegoProcessAssociationsRepo.Sync(cdpas)

	return &api.BulkSyncResponse{}, nil
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

func validateUpsertCAPIDiegoProcessAssociationRequest(r *api.UpsertCapiDiegoProcessAssociationRequest) error {
	association := r.CapiDiegoProcessAssociation
	if association == nil {
		return errors.New("CapiDiegoProcessAssociation is required")
	}
	if association.CapiProcessGuid == "" || len(association.DiegoProcessGuids) == 0 {
		return errors.New("CapiProcessGuid and DiegoProcessGuids are required")
	}
	return nil
}

func validateDeleteCAPIDiegoProcessAssociationRequest(r *api.DeleteCapiDiegoProcessAssociationRequest) error {
	if r.CapiProcessGuid == "" {
		return errors.New("CapiProcessGuid is required")
	}
	return nil
}
