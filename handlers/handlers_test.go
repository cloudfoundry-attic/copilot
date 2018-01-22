package handlers_test

import (
	"context"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/handlers"
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

var _ = Describe("Handlers", func() {
	var (
		handler           *handlers.Copilot
		bbsClient         *mockBBSClient
		logger            lager.Logger
		bbsClientResponse []*bbsmodels.ActualLRPGroup
		backendSetA       *api.BackendSet
		backendSetB       *api.BackendSet
	)

	BeforeEach(func() {
		bbsClientResponse = []*bbsmodels.ActualLRPGroup{
			&bbsmodels.ActualLRPGroup{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-a", 1, "domain1"),
					State:        bbsmodels.ActualLRPStateRunning,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.10.1.5",
						Ports: []*bbsmodels.PortMapping{
							&bbsmodels.PortMapping{ContainerPort: 2222, HostPort: 61006},
							&bbsmodels.PortMapping{ContainerPort: 8080, HostPort: 61005},
						},
					},
				},
			},
			&bbsmodels.ActualLRPGroup{},
			&bbsmodels.ActualLRPGroup{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-a", 2, "domain1"),
					State:        bbsmodels.ActualLRPStateRunning,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.0.40.2",
						Ports: []*bbsmodels.PortMapping{
							&bbsmodels.PortMapping{ContainerPort: 8080, HostPort: 61008},
						},
					},
				},
			},
			&bbsmodels.ActualLRPGroup{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-b", 1, "domain1"),
					State:        bbsmodels.ActualLRPStateClaimed,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.0.40.4",
						Ports: []*bbsmodels.PortMapping{
							&bbsmodels.PortMapping{ContainerPort: 8080, HostPort: 61007},
						},
					},
				},
			},
			&bbsmodels.ActualLRPGroup{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-b", 1, "domain1"),
					State:        bbsmodels.ActualLRPStateRunning,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.0.50.4",
						Ports: []*bbsmodels.PortMapping{
							&bbsmodels.PortMapping{ContainerPort: 8080, HostPort: 61009},
						},
					},
				},
			},
			&bbsmodels.ActualLRPGroup{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-b", 2, "domain1"),
					State:        bbsmodels.ActualLRPStateRunning,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.0.60.2",
						Ports: []*bbsmodels.PortMapping{
							&bbsmodels.PortMapping{ContainerPort: 8080, HostPort: 61001},
						},
					},
				},
			},
		}

		backendSetA = &api.BackendSet{
			Backends: []*api.Backend{
				{
					Address: "10.10.1.5",
					Port:    61005,
				},
				{
					Address: "10.0.40.2",
					Port:    61008,
				},
			},
		}
		backendSetB = &api.BackendSet{
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

		bbsClient = &mockBBSClient{
			actualLRPGroupsData: bbsClientResponse,
		}

		logger = lagertest.NewTestLogger("test")

		handler = &handlers.Copilot{
			BBSClient:         bbsClient,
			Logger:            logger,
			RoutesRepo:        handlers.RoutesRepo(make(map[handlers.RouteGUID]*handlers.Route)),
			RouteMappingsRepo: handlers.RouteMappingsRepo(make(map[string]*handlers.RouteMapping)),
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
		It("returns the routes for each running backend instance", func() {
			handler.RoutesRepo[handlers.RouteGUID("route-guid-a")] = &handlers.Route{
				GUID:     "route-guid-a",
				Hostname: "route-a.cfapps.com",
			}
			routeMapping := &handlers.RouteMapping{
				RouteGUID: "route-guid-a",
				Process: &handlers.Process{
					GUID: "process-guid-a",
				},
			}
			handler.RouteMappingsRepo[routeMapping.Key()] = routeMapping
			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"process-guid-a.cfapps.internal": backendSetA,
					"process-guid-b.cfapps.internal": backendSetB,
					"route-a.cfapps.com":             backendSetA,
				},
			}))
		})

		It("ignores route mappings for routes that do not exist", func() {
			routeMapping := &handlers.RouteMapping{
				RouteGUID: "route-guid-a",
				Process: &handlers.Process{
					GUID: "process-guid-a",
				},
			}
			handler.RouteMappingsRepo[routeMapping.Key()] = routeMapping
			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"process-guid-a.cfapps.internal": backendSetA,
					"process-guid-b.cfapps.internal": backendSetB,
				},
			}))
		})
	})

	Describe("MapRoute", func() {
		BeforeEach(func() {
			handler.RoutesRepo[handlers.RouteGUID("route-guid-a")] = &handlers.Route{
				GUID:     "route-guid-a",
				Hostname: "route-a.example.com",
			}
		})

		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.MapRoute(ctx, &api.MapRouteRequest{RouteGuid: "some-route-guid"})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.MapRoute(ctx, &api.MapRouteRequest{Process: &api.Process{Guid: "some-process-guid"}})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("maps the route", func() {
			ctx := context.Background()
			_, err := handler.MapRoute(ctx, &api.MapRouteRequest{
				RouteGuid: "route-guid-a",
				Process: &api.Process{
					Guid: "process-guid-a",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"process-guid-a.cfapps.internal": backendSetA,
					"process-guid-b.cfapps.internal": backendSetB,
					"route-a.example.com":            backendSetA,
				},
			}))
		})

		It("maps one route to multiple processes", func() {
			ctx := context.Background()
			_, err := handler.MapRoute(ctx, &api.MapRouteRequest{
				RouteGuid: "route-guid-a",
				Process: &api.Process{
					Guid: "process-guid-a",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = handler.MapRoute(ctx, &api.MapRouteRequest{
				RouteGuid: "route-guid-a",
				Process: &api.Process{
					Guid: "process-guid-b",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Backends["process-guid-a.cfapps.internal"].Backends).To(ConsistOf(backendSetA.Backends))
			Expect(resp.Backends["process-guid-b.cfapps.internal"].Backends).To(ConsistOf(backendSetB.Backends))
			Expect(resp.Backends["route-a.example.com"].Backends).To(ConsistOf(
				append(backendSetA.Backends, backendSetB.Backends...),
			))
		})

		It("only creates one route mapping when the same route and process are mapped twice", func() {
			ctx := context.Background()
			_, err := handler.MapRoute(ctx, &api.MapRouteRequest{
				RouteGuid: "route-guid-a",
				Process: &api.Process{
					Guid: "process-guid-a",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = handler.MapRoute(ctx, &api.MapRouteRequest{
				RouteGuid: "route-guid-a",
				Process: &api.Process{
					Guid: "process-guid-a",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"process-guid-a.cfapps.internal": backendSetA,
					"process-guid-b.cfapps.internal": backendSetB,
					"route-a.example.com":            backendSetA,
				},
			}))
		})
	})

	Describe("UnmapRoute", func() {
		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.UnmapRoute(ctx, &api.UnmapRouteRequest{RouteGuid: "some-route-guid"})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.UnmapRoute(ctx, &api.UnmapRouteRequest{ProcessGuid: "some-process-guid"})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("unmaps the routes", func() {
			ctx := context.Background()
			_, err := handler.MapRoute(ctx, &api.MapRouteRequest{RouteGuid: "to-be-deleted-route-guid", Process: &api.Process{Guid: "process-guid-a"}})
			Expect(err).NotTo(HaveOccurred())
			_, err = handler.UnmapRoute(ctx, &api.UnmapRouteRequest{RouteGuid: "to-be-deleted-route-guid", ProcessGuid: "process-guid-a"})
			Expect(err).NotTo(HaveOccurred())
			resp, err := handler.Routes(ctx, nil)
			Expect(resp.Backends["to-be-deleted-host"]).To(BeNil())
		})

		It("does not error when the route does not exist", func() {
			ctx := context.Background()
			_, err := handler.UnmapRoute(ctx, &api.UnmapRouteRequest{RouteGuid: "does-not-exist", ProcessGuid: "process-guid-does-not-exist"})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
