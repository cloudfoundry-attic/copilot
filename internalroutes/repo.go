package internalroutes

import (
	"strings"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
)

const CF_APP_SSH_PORT = 2222

type routesRepoInterface interface {
	Upsert(route *models.Route)
	Delete(guid models.RouteGUID)
	Sync(routes []*models.Route)
	Get(guid models.RouteGUID) (*models.Route, bool)
	List() map[string]string
}

type routeMappingsRepoInterface interface {
	Map(routeMapping *models.RouteMapping)
	Unmap(routeMapping *models.RouteMapping)
	Sync(routeMappings []*models.RouteMapping)
	List() map[string]*models.RouteMapping
}

type capiDiegoProcessAssociationsRepoInterface interface {
	Upsert(capiDiegoProcessAssociation *models.CAPIDiegoProcessAssociation)
	Delete(capiProcessGUID *models.CAPIProcessGUID)
	Sync(capiDiegoProcessAssociations []*models.CAPIDiegoProcessAssociation)
	List() map[models.CAPIProcessGUID]*models.DiegoProcessGUIDs
	Get(capiProcessGUID *models.CAPIProcessGUID) *models.CAPIDiegoProcessAssociation
}

//go:generate counterfeiter -o fakes/backendset_repo.go --fake-name BackendSetRepo . backendSetRepo
type backendSetRepo interface {
	GetInternalBackends(guid models.DiegoProcessGUID) *api.BackendSet
}

//go:generate counterfeiter -o fakes/vip_provider.go --fake-name VIPProvider . vipProvider
type vipProvider interface {
	Get(hostname string) string
}

type Repo struct {
	BackendSetRepo                   backendSetRepo
	Logger                           lager.Logger
	RoutesRepo                       routesRepoInterface
	RouteMappingsRepo                routeMappingsRepoInterface
	CAPIDiegoProcessAssociationsRepo capiDiegoProcessAssociationsRepoInterface
	VIPProvider                      vipProvider
}

type Backend struct {
	Address string
	Port    uint32
}

type InternalRoute struct {
	Hostname string
	VIP      string
}

func (r *Repo) Get() (map[InternalRoute][]Backend, error) {
	hostnamesToBackends := map[string][]Backend{}

	for _, routeMapping := range r.RouteMappingsRepo.List() {
		route, ok := r.RoutesRepo.Get(routeMapping.RouteGUID)
		if !ok {
			continue
		}

		hostname := route.Hostname()
		if !strings.HasSuffix(hostname, ".apps.internal") {
			continue
		}

		capiDiegoProcessAssociation := r.CAPIDiegoProcessAssociationsRepo.Get(&routeMapping.CAPIProcessGUID)
		if capiDiegoProcessAssociation == nil {
			continue
		}

		allBackendsForThisRouteMapping := []Backend{}
		for _, diegoProcessGUID := range capiDiegoProcessAssociation.DiegoProcessGUIDs {
			bs := r.BackendSetRepo.GetInternalBackends(diegoProcessGUID)

			for _, backend := range bs.Backends {
				if backend.Port == 0 {
					continue
				}

				allBackendsForThisRouteMapping = append(allBackendsForThisRouteMapping, Backend{
					Address: backend.Address,
					Port:    backend.Port,
				})
			}
		}

		hostnamesToBackends[hostname] = append(hostnamesToBackends[hostname], allBackendsForThisRouteMapping...)
	}

	result := map[InternalRoute][]Backend{}
	for hostname, backends := range hostnamesToBackends {
		vip := r.VIPProvider.Get(hostname)
		result[InternalRoute{
			Hostname: hostname,
			VIP:      vip,
		}] = backends
	}

	return result, nil
}
