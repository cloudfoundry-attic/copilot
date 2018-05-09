package internalroutes

import (
	"strings"

	bbsmodels "code.cloudfoundry.org/bbs/models"
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

//go:generate counterfeiter -o fakes/bbs_client.go --fake-name BBSClient . bbsClient
type bbsClient interface {
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

//go:generate counterfeiter -o fakes/vip_provider.go --fake-name VIPProvider . vipProvider
type vipProvider interface {
	Get(hostname string) string
}

type Repo struct {
	BBSClient                        bbsClient
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
	lrpNetInfosMap, err := r.retrieveActualLRPNetInfos()
	if err != nil {
		return nil, err
	}

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
			for _, lrpNetInfo := range lrpNetInfosMap[diegoProcessGUID] {
				appContainerPort := getAppContainerPort(lrpNetInfo)
				if appContainerPort == 0 {
					continue
				}
				allBackendsForThisRouteMapping = append(allBackendsForThisRouteMapping, Backend{
					Address: lrpNetInfo.InstanceAddress,
					Port:    appContainerPort,
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

func (r *Repo) retrieveActualLRPNetInfos() (map[models.DiegoProcessGUID][]bbsmodels.ActualLRPNetInfo, error) {
	actualLRPGroups, err := r.BBSClient.ActualLRPGroups(r.Logger.Session("bbs-client"), bbsmodels.ActualLRPFilter{})
	if err != nil {
		return nil, err
	}
	actualLRPNetInfos := make(map[models.DiegoProcessGUID][]bbsmodels.ActualLRPNetInfo)
	for _, actualGroup := range actualLRPGroups {
		instance := actualGroup.Instance
		if instance == nil {
			r.Logger.Debug("skipping-nil-instance")
			continue
		}
		diegoProcessGUID := models.DiegoProcessGUID(instance.ActualLRPKey.ProcessGuid)
		if instance.State != bbsmodels.ActualLRPStateRunning {
			r.Logger.Debug("skipping-non-running-instance", lager.Data{"process-guid": diegoProcessGUID})
			continue
		}
		netInfos := actualLRPNetInfos[diegoProcessGUID]
		netInfos = append(netInfos, instance.ActualLRPNetInfo)
		actualLRPNetInfos[diegoProcessGUID] = netInfos
	}
	return actualLRPNetInfos, nil
}

func getAppContainerPort(lrpNetInfo bbsmodels.ActualLRPNetInfo) uint32 {
	for _, port := range lrpNetInfo.Ports {
		if port.ContainerPort != CF_APP_SSH_PORT {
			return port.ContainerPort
		}
	}
	return 0
}
