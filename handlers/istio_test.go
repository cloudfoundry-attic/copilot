package handlers_test

import (
	"context"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/internalroutes"
	internalroutes_fakes "code.cloudfoundry.org/copilot/internalroutes/fakes"
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

var _ = Describe("Istio Handlers", func() {
	var (
		handler                        *handlers.Istio
		bbsClient                      *mockBBSClient
		logger                         lager.Logger
		bbsClientResponse              []*bbsmodels.ActualLRPGroup
		expectedExternalRouteBackendsA *api.BackendSet
		expectedExternalRouteBackendsB *api.BackendSet
		expectedInternalRouteBackendsA *api.BackendSet
		expectedInternalRouteBackendsB *api.BackendSet
		vipProvider                    *internalroutes_fakes.VIPProvider
	)

	BeforeEach(func() {
		bbsClientResponse = []*bbsmodels.ActualLRPGroup{
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

		expectedExternalRouteBackendsA = &api.BackendSet{
			Backends: []*api.Backend{
				{
					Address: "10.0.40.2",
					Port:    61008,
				},
				{
					Address: "10.10.1.5",
					Port:    61005,
				},
			},
		}
		expectedExternalRouteBackendsB = &api.BackendSet{
			Backends: []*api.Backend{
				{
					Address: "10.0.50.4",
					Port:    61009,
				},
				{
					Address: "10.0.60.2",
					Port:    61001,
				},
			},
		}

		expectedInternalRouteBackendsA = &api.BackendSet{
			Backends: []*api.Backend{
				{
					Address: "10.255.0.16",
					Port:    8080,
				},
				{
					Address: "10.255.1.34",
					Port:    9080,
				},
			},
		}
		expectedInternalRouteBackendsB = &api.BackendSet{
			Backends: []*api.Backend{
				{
					Address: "10.255.9.16",
					Port:    8080,
				},
				{
					Address: "10.255.9.34",
					Port:    8080,
				},
			},
		}

		bbsClient = &mockBBSClient{
			actualLRPGroupsData: bbsClientResponse,
		}

		logger = lagertest.NewTestLogger("test")

		vipProvider = &internalroutes_fakes.VIPProvider{}
		vipProvider.GetStub = func(hostname string) string {
			return map[string]string{
				"route-a.apps.internal": "vip-for-route-a",
				"route-b.apps.internal": "vip-for-route-b",
			}[hostname]
		}

		routesRepo := &models.RoutesRepo{
			Repo: make(map[models.RouteGUID]*models.Route),
		}
		routeMappingsRepo := &models.RouteMappingsRepo{
			Repo: make(map[string]*models.RouteMapping),
		}
		capiDiegoProcessAssociationsRepo := &models.CAPIDiegoProcessAssociationsRepo{
			Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
		}

		internalRoutesRepo := &internalroutes.Repo{
			BBSClient:                        bbsClient,
			Logger:                           logger,
			RoutesRepo:                       routesRepo,
			RouteMappingsRepo:                routeMappingsRepo,
			CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
			VIPProvider:                      vipProvider,
		}

		handler = &handlers.Istio{
			BBSClient:                        bbsClient,
			Logger:                           logger,
			RoutesRepo:                       routesRepo,
			RouteMappingsRepo:                routeMappingsRepo,
			CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
			InternalRoutesRepo:               internalRoutesRepo,
		}
	})

	Describe("Health", func() {
		It("always returns healthy", func() {
			ctx := context.Background()
			resp, err := handler.Health(ctx, new(api.HealthRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.HealthResponse{Healthy: true}))
		})
	})

	Describe("listing InternalRoutes (using real repos, to cover more intration-y things", func() {
		BeforeEach(func() {
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "internal-route-guid-a",
				Host: "route-a.apps.internal",
			})
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "internal-route-guid-b",
				Host: "route-b.apps.internal",
			})
			handler.RouteMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-a",
				CAPIProcessGUID: "capi-process-guid-a",
			})
			handler.RouteMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-b",
				CAPIProcessGUID: "capi-process-guid-b",
			})
			handler.CAPIDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-a",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-a",
				},
			})
			handler.CAPIDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-b",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-b",
				},
			})
		})

		It("returns the internal routes for each running backend instance", func() {
			ctx := context.Background()
			externalRouteResp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(externalRouteResp.Routes).To(HaveLen(0))

			resp, err := handler.InternalRoutes(ctx, new(api.InternalRoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.InternalRoutes).To(HaveLen(2))

			receivedInternalRoutes := make(map[string]*api.InternalRouteWithBackends)
			for _, ir := range resp.InternalRoutes {
				receivedInternalRoutes[ir.Hostname] = ir
			}
			Expect(receivedInternalRoutes).To(HaveKey("route-a.apps.internal"))
			Expect(receivedInternalRoutes).To(HaveKey("route-b.apps.internal"))

			internalRoute0 := receivedInternalRoutes["route-a.apps.internal"]
			Expect(internalRoute0.Hostname).To(Equal("route-a.apps.internal"))
			Expect(internalRoute0.Vip).To(Equal("vip-for-route-a"))
			Expect(internalRoute0.Backends.Backends).To(ConsistOf(expectedInternalRouteBackendsA.Backends))

			internalRoute1 := receivedInternalRoutes["route-b.apps.internal"]
			Expect(internalRoute1.Hostname).To(Equal("route-b.apps.internal"))
			Expect(internalRoute1.Vip).To(Equal("vip-for-route-b"))
			Expect(internalRoute1.Backends.Backends).To(ConsistOf(expectedInternalRouteBackendsB.Backends))

			// and now map the route-a to diego-process-b
			// and assert we see all 4 backends for route-a
			handler.RouteMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-a",
				CAPIProcessGUID: "capi-process-guid-b",
			})

			externalRouteResp, err = handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(externalRouteResp.Routes).To(HaveLen(0))

			resp, err = handler.InternalRoutes(ctx, new(api.InternalRoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.InternalRoutes).To(HaveLen(2))

			receivedInternalRoutes = make(map[string]*api.InternalRouteWithBackends)
			for _, ir := range resp.InternalRoutes {
				receivedInternalRoutes[ir.Hostname] = ir
			}
			Expect(receivedInternalRoutes).To(HaveKey("route-a.apps.internal"))
			Expect(receivedInternalRoutes).To(HaveKey("route-b.apps.internal"))

			internalRoute0 = receivedInternalRoutes["route-a.apps.internal"]
			Expect(internalRoute0.Hostname).To(Equal("route-a.apps.internal"))
			Expect(internalRoute0.Vip).To(Equal("vip-for-route-a"))
			Expect(internalRoute0.Backends.Backends).To(ConsistOf(
				append(expectedInternalRouteBackendsA.Backends, expectedInternalRouteBackendsB.Backends...),
			))
		})
	})

	Describe("listing Routes (using real repos, to cover more integration-y things)", func() {
		BeforeEach(func() {
			handler.RouteMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "route-guid-a",
				CAPIProcessGUID: "capi-process-guid-a",
			})
			handler.RouteMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "route-guid-b",
				CAPIProcessGUID: "capi-process-guid-b",
			})
			handler.CAPIDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-a",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-a",
				},
			})
			handler.CAPIDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-b",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-b",
				},
			})
		})

		It("returns the sorted routes for each running backend instance", func() {
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "route-guid-a",
				Host: "ROUTE-a.cfapps.com",
			})
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "route-guid-b",
				Host: "route-b.cfapps.com",
				Path: "/some/path",
			})
			ctx := context.Background()

			internalRouteResp, err := handler.InternalRoutes(ctx, new(api.InternalRoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(internalRouteResp.InternalRoutes).To(HaveLen(0))

			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Routes).To(HaveLen(2))
			Expect(resp.Routes).To(Equal([]*api.RouteWithBackends{
				&api.RouteWithBackends{
					Hostname:        "route-b.cfapps.com",
					Backends:        expectedExternalRouteBackendsB,
					Path:            "/some/path",
					CapiProcessGuid: "capi-process-guid-b",
				},
				&api.RouteWithBackends{
					Hostname:        "route-a.cfapps.com",
					Backends:        expectedExternalRouteBackendsA,
					CapiProcessGuid: "capi-process-guid-a",
				},
			},
			))
		})

		It("ignores route mappings for routes that do not exist", func() {
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "route-guid-a",
				Host: "ROUTE-a.cfapps.com",
			})
			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Routes: []*api.RouteWithBackends{
					&api.RouteWithBackends{
						Hostname:        "route-a.cfapps.com",
						Backends:        expectedExternalRouteBackendsA,
						CapiProcessGuid: "capi-process-guid-a",
					},
				},
			}))
		})

		It("sorts routes with multiple context paths from shortest to longest path", func() {
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "route-guid-a",
				Host: "route-a.cfapps.com",
			})
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "route-guid-b",
				Host: "route-a.cfapps.com",
				Path: "/zxyv/some/longer/path",
			})
			handler.RoutesRepo.Upsert(&models.Route{
				GUID: "route-guid-c",
				Host: "route-a.cfapps.com",
				Path: "/some/path",
			})
			handler.RouteMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "route-guid-c",
				CAPIProcessGUID: "capi-process-guid-b",
			})

			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Routes).To(HaveLen(3))
			Expect(resp.Routes).To(Equal([]*api.RouteWithBackends{
				&api.RouteWithBackends{
					Hostname:        "route-a.cfapps.com",
					Backends:        expectedExternalRouteBackendsB,
					Path:            "/some/path",
					CapiProcessGuid: "capi-process-guid-b",
				},
				&api.RouteWithBackends{
					Hostname:        "route-a.cfapps.com",
					Backends:        expectedExternalRouteBackendsB,
					Path:            "/zxyv/some/longer/path",
					CapiProcessGuid: "capi-process-guid-b",
				},
				&api.RouteWithBackends{
					Hostname:        "route-a.cfapps.com",
					Backends:        expectedExternalRouteBackendsA,
					CapiProcessGuid: "capi-process-guid-a",
				},
			},
			))
		})

		Context("when the BBSClient is nil (BBS has been disabled)", func() {
			BeforeEach(func() {
				handler.BBSClient = nil
			})

			It("returns a helpful error", func() {
				ctx := context.Background()
				_, err := handler.Routes(ctx, new(api.RoutesRequest))
				Expect(err).To(MatchError("communication with bbs is disabled"))
			})
		})
	})
})
