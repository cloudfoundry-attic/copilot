package internal_routes_test

import (
	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/internal_routes"
	"code.cloudfoundry.org/copilot/internal_routes/fakes"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockBBSClient struct {
	actualLRPGroupsData []*bbsmodels.ActualLRPGroup
	actualLRPErr        error
}

func (b mockBBSClient) ActualLRPGroups(l lager.Logger, bbsModel bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error) {
	return b.actualLRPGroupsData, b.actualLRPErr
}

var _ = Describe("Repo", func() {
	Describe("Get", func() {
		var (
			routesRepo                       *models.RoutesRepo
			routeMappingsRepo                *models.RouteMappingsRepo
			capiDiegoProcessAssociationsRepo *models.CAPIDiegoProcessAssociationsRepo
			bbsClient                        *mockBBSClient
			logger                           lager.Logger
			vipProvider                      *fakes.VIPProvider
			internalRoutesRepo               *internal_routes.Repo
		)

		BeforeEach(func() {
			bbsClientResponse := []*bbsmodels.ActualLRPGroup{
				{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-a", 1, "domain1"),
						State:        bbsmodels.ActualLRPStateRunning,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address:         "10.10.1.5",
							InstanceAddress: "10.255.0.16",
							Ports: []*bbsmodels.PortMapping{
								{ContainerPort: 2222, HostPort: 61006},
								{ContainerPort: 8080, HostPort: 61005},
							},
						},
					},
				},
				{},
				{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-a", 2, "domain1"),
						State:        bbsmodels.ActualLRPStateRunning,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address:         "10.0.40.2",
							InstanceAddress: "10.255.1.34",
							Ports: []*bbsmodels.PortMapping{
								{ContainerPort: 9080, HostPort: 61008},
							},
						},
					},
				},
				{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-b", 1, "domain1"),
						State:        bbsmodels.ActualLRPStateClaimed, // not yet started
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address:         "10.0.40.4",
							InstanceAddress: "10.255.7.77",
							Ports: []*bbsmodels.PortMapping{
								{ContainerPort: 8080, HostPort: 61007},
							},
						},
					},
				},
				{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-b", 1, "domain1"),
						State:        bbsmodels.ActualLRPStateRunning, // actually running
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address:         "10.0.50.4",
							InstanceAddress: "10.255.9.16",
							Ports: []*bbsmodels.PortMapping{
								{ContainerPort: 8080, HostPort: 61009},
							},
						},
					},
				},
				{
					Instance: &bbsmodels.ActualLRP{
						ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-b", 2, "domain1"),
						State:        bbsmodels.ActualLRPStateRunning,
						ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
							Address:         "10.0.60.2",
							InstanceAddress: "10.255.9.34",
							Ports: []*bbsmodels.PortMapping{
								{ContainerPort: 8080, HostPort: 61001},
							},
						},
					},
				},
			}
			bbsClient = &mockBBSClient{
				actualLRPGroupsData: bbsClientResponse,
			}

			logger = lagertest.NewTestLogger("test")

			routesRepo = &models.RoutesRepo{
				Repo: make(map[models.RouteGUID]*models.Route),
			}
			routeMappingsRepo = &models.RouteMappingsRepo{
				Repo: make(map[string]*models.RouteMapping),
			}
			capiDiegoProcessAssociationsRepo = &models.CAPIDiegoProcessAssociationsRepo{
				Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
			}
			routesRepo.Upsert(&models.Route{
				GUID: "internal-route-guid-a",
				Host: "route-a.apps.internal",
			})
			routesRepo.Upsert(&models.Route{
				GUID: "internal-route-guid-b",
				Host: "route-b.apps.internal",
			})
			routeMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-a",
				CAPIProcessGUID: "capi-process-guid-a",
			})
			routeMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-b",
				CAPIProcessGUID: "capi-process-guid-b",
			})
			capiDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-a",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-a",
				},
			})
			capiDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-b",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-b",
				},
			})
			vipProvider = &fakes.VIPProvider{}
			vipProvider.GetStub = func(hostname string) string {
				return map[string]string{
					"route-a.apps.internal": "vip-for-route-a",
					"route-b.apps.internal": "vip-for-route-b",
				}[hostname]
			}

			internalRoutesRepo = &internal_routes.Repo{
				BBSClient:                        bbsClient,
				Logger:                           logger,
				RoutesRepo:                       routesRepo,
				RouteMappingsRepo:                routeMappingsRepo,
				CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
				VIPProvider:                      vipProvider,
			}
		})

		It("returns the internal routes for each running backend instance", func() {
			routeAKey := internal_routes.InternalRoute{Hostname: "route-a.apps.internal", VIP: "vip-for-route-a"}
			routeBKey := internal_routes.InternalRoute{Hostname: "route-b.apps.internal", VIP: "vip-for-route-b"}

			internalRoutes, err := internalRoutesRepo.Get()
			Expect(err).NotTo(HaveOccurred())
			Expect(internalRoutes).To(HaveLen(2))

			Expect(internalRoutes).To(HaveKey(routeAKey))
			Expect(internalRoutes[routeAKey]).To(ConsistOf([]internal_routes.Backend{
				{
					Address: "10.255.0.16",
					Port:    8080,
				},
				{
					Address: "10.255.1.34",
					Port:    9080,
				},
			}))

			Expect(internalRoutes).To(HaveKey(routeBKey))
			Expect(internalRoutes[routeBKey]).To(ConsistOf([]internal_routes.Backend{
				{
					Address: "10.255.9.16",
					Port:    8080,
				},
				{
					Address: "10.255.9.34",
					Port:    8080,
				},
			}))

			// and now map the route-a to diego-process-b
			// and assert we see all 4 backends for route-a
			routeMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-a",
				CAPIProcessGUID: "capi-process-guid-b",
			})

			internalRoutes, err = internalRoutesRepo.Get()
			Expect(err).NotTo(HaveOccurred())
			Expect(internalRoutes).To(HaveLen(2))

			Expect(internalRoutes).To(HaveKey(routeAKey))
			Expect(internalRoutes[routeAKey]).To(ConsistOf([]internal_routes.Backend{
				{
					Address: "10.255.0.16",
					Port:    8080,
				},
				{
					Address: "10.255.1.34",
					Port:    9080,
				},
				{
					Address: "10.255.9.16",
					Port:    8080,
				},
				{
					Address: "10.255.9.34",
					Port:    8080,
				},
			}))

			Expect(internalRoutes).To(HaveKey(routeBKey))
			Expect(internalRoutes[routeBKey]).To(ConsistOf([]internal_routes.Backend{
				{
					Address: "10.255.9.16",
					Port:    8080,
				},
				{
					Address: "10.255.9.34",
					Port:    8080,
				},
			}))
		})
	})
})
