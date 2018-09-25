package handlers_test

import (
	"context"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/handlers/fakes"
	"code.cloudfoundry.org/copilot/internalroutes"
	internalroutesfakes "code.cloudfoundry.org/copilot/internalroutes/fakes"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Istio Handlers", func() {
	var (
		handler                        *handlers.Istio
		backendSetRepo                 *fakes.BackendSetRepo
		collector                      *fakes.Collector
		logger                         lager.Logger
		expectedInternalRouteBackendsA *api.BackendSet
		expectedInternalRouteBackendsB *api.BackendSet
		vipProvider                    *internalroutesfakes.VIPProvider
	)

	BeforeEach(func() {
		backendSetRepo = &fakes.BackendSetRepo{}
		backendSetRepo.GetInternalBackendsStub = func(guid models.DiegoProcessGUID) *api.BackendSet {
			diegoClientMap := map[models.DiegoProcessGUID]*api.BackendSet{
				"diego-process-guid-b": &api.BackendSet{
					Backends: []*api.Backend{
						{Address: "10.255.9.34", Port: 8080},
						{Address: "10.255.9.16", Port: 8080},
					},
				},
				"diego-process-guid-a": &api.BackendSet{
					Backends: []*api.Backend{
						{Address: "10.255.0.16", Port: 8080},
						{Address: "10.255.1.34", Port: 9080},
					},
				},
			}
			return diegoClientMap[guid]
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

		logger = lagertest.NewTestLogger("test")
		collector = &fakes.Collector{}

		vipProvider = &internalroutesfakes.VIPProvider{}
		vipProvider.GetStub = func(hostname string) string {
			return map[string]string{
				"route-a.apps.internal": "vip-for-route-a",
				"route-b.apps.internal": "vip-for-route-b",
			}[hostname]
		}

		routesRepo := models.NewRoutesRepo()
		routeMappingsRepo := models.NewRouteMappingsRepo()
		capiDiegoProcessAssociationsRepo := &models.CAPIDiegoProcessAssociationsRepo{
			Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
		}

		internalRoutesRepo := &internalroutes.Repo{
			BackendSetRepo:                   backendSetRepo,
			Logger:                           logger,
			RoutesRepo:                       routesRepo,
			RouteMappingsRepo:                routeMappingsRepo,
			CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
			VIPProvider:                      vipProvider,
		}

		handler = &handlers.Istio{
			Logger:                           logger,
			Collector:                        collector,
			BackendSetRepo:                   backendSetRepo,
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

	Describe("Routes", func() {
		var routesResponse []*api.RouteWithBackends

		BeforeEach(func() {
			routesResponse = []*api.RouteWithBackends{
				&api.RouteWithBackends{
					Hostname: "route-a.cfapps.com",
					Backends: &api.BackendSet{
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
					},
					CapiProcessGuid: "capi-process-guid-a",
					RouteWeight:     67,
				},
			}

			collector.CollectReturns(routesResponse)
		})

		It("returns the routes from the collector", func() {
			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Routes: routesResponse,
			}))
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
})
