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

		bbsClient = &mockBBSClient{
			actualLRPGroupsData: bbsClientResponse,
		}
		logger = lagertest.NewTestLogger("test")
		handler = &handlers.Copilot{
			BBSClient:  bbsClient,
			Logger:     logger,
			RoutesRepo: make(map[string]*handlers.RouteMapping),
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
		It("returns a route for each running backend instance", func() {
			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"process-guid-a.cfapps.internal": &api.BackendSet{
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
					},
					"process-guid-b.cfapps.internal": &api.BackendSet{
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
					},
				},
			}))
		})
	})

	Describe("AddRoute", func() {
		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.AddRoute(ctx, &api.AddRouteRequest{Hostname: "some-host"})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.AddRoute(ctx, &api.AddRouteRequest{ProcessGuid: "some-guid"})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("adds the route", func() {
			ctx := context.Background()
			_, err := handler.AddRoute(ctx, &api.AddRouteRequest{
				ProcessGuid: "process-guid-a",
				Hostname:    "app-a.example.com",
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"process-guid-a.cfapps.internal": &api.BackendSet{
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
					},
					"process-guid-b.cfapps.internal": &api.BackendSet{
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
					},
					"app-a.example.com": &api.BackendSet{
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
					},
				},
			}))
		})

		It("adds routes with overlapping hostnames", func() {
			ctx := context.Background()
			_, err := handler.AddRoute(ctx, &api.AddRouteRequest{
				ProcessGuid: "process-guid-a",
				Hostname:    "app-a.example.com",
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = handler.AddRoute(ctx, &api.AddRouteRequest{
				ProcessGuid: "process-guid-b",
				Hostname:    "app-a.example.com",
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Backends["process-guid-a.cfapps.internal"].Backends).To(ConsistOf(
				[]*api.Backend{
					{
						Address: "10.10.1.5",
						Port:    61005,
					},
					{
						Address: "10.0.40.2",
						Port:    61008,
					},
				}))
			Expect(resp.Backends["process-guid-b.cfapps.internal"].Backends).To(ConsistOf(
				[]*api.Backend{
					{
						Address: "10.0.50.4",
						Port:    61009,
					},
					{
						Address: "10.0.60.2",
						Port:    61001,
					},
				},
			))
			Expect(resp.Backends["app-a.example.com"].Backends).To(ConsistOf(
				[]*api.Backend{
					{
						Address: "10.10.1.5",
						Port:    61005,
					},
					{
						Address: "10.0.40.2",
						Port:    61008,
					},
					{
						Address: "10.0.50.4",
						Port:    61009,
					},
					{
						Address: "10.0.60.2",
						Port:    61001,
					},
				}))
		})

		It("adding the same route twice only returns once", func() {
			ctx := context.Background()
			_, err := handler.AddRoute(ctx, &api.AddRouteRequest{
				ProcessGuid: "process-guid-a",
				Hostname:    "app-a.example.com",
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = handler.AddRoute(ctx, &api.AddRouteRequest{
				ProcessGuid: "process-guid-a",
				Hostname:    "app-a.example.com",
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"process-guid-a.cfapps.internal": &api.BackendSet{
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
					},
					"process-guid-b.cfapps.internal": &api.BackendSet{
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
					},
					"app-a.example.com": &api.BackendSet{
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
					},
				},
			}))
		})
	})

	Describe("DeleteRoute", func() {
		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.DeleteRoute(ctx, &api.DeleteRouteRequest{Hostname: "some-host"})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.DeleteRoute(ctx, &api.DeleteRouteRequest{ProcessGuid: "some-guid"})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("deletes the routes", func() {
			ctx := context.Background()
			_, err := handler.AddRoute(ctx, &api.AddRouteRequest{Hostname: "to-be-deleted-host", ProcessGuid: "process-guid-a"})
			Expect(err).NotTo(HaveOccurred())
			_, err = handler.DeleteRoute(ctx, &api.DeleteRouteRequest{Hostname: "to-be-deleted-host", ProcessGuid: "process-guid-a"})
			Expect(err).NotTo(HaveOccurred())
			resp, err := handler.Routes(ctx, nil)
			Expect(resp.Backends["to-be-deleted-host"]).To(BeNil())
		})

		It("does not error when the route does not exist", func() {
			ctx := context.Background()
			_, err := handler.DeleteRoute(ctx, &api.DeleteRouteRequest{Hostname: "does-not-exist", ProcessGuid: "process-guid-does-not-exist"})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
