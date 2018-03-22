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

var _ = Describe("Istio Handlers", func() {
	var (
		handler           *handlers.Istio
		bbsClient         *mockBBSClient
		logger            lager.Logger
		bbsClientResponse []*bbsmodels.ActualLRPGroup
		backendSetA       *api.BackendSet
		backendSetB       *api.BackendSet
	)

	BeforeEach(func() {
		bbsClientResponse = []*bbsmodels.ActualLRPGroup{
			{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-a", 1, "domain1"),
					State:        bbsmodels.ActualLRPStateRunning,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.10.1.5",
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
						Address: "10.0.40.2",
						Ports: []*bbsmodels.PortMapping{
							{ContainerPort: 8080, HostPort: 61008},
						},
					},
				},
			},
			{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-b", 1, "domain1"),
					State:        bbsmodels.ActualLRPStateClaimed,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.0.40.4",
						Ports: []*bbsmodels.PortMapping{
							{ContainerPort: 8080, HostPort: 61007},
						},
					},
				},
			},
			{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-b", 1, "domain1"),
					State:        bbsmodels.ActualLRPStateRunning,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address: "10.0.50.4",
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
						Address: "10.0.60.2",
						Ports: []*bbsmodels.PortMapping{
							{ContainerPort: 8080, HostPort: 61001},
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

		handler = &handlers.Istio{
			BBSClient: bbsClient,
			Logger:    logger,
			RoutesRepo: &handlers.RoutesRepo{
				Repo: make(map[handlers.RouteGUID]*handlers.Route),
			},
			RouteMappingsRepo: &handlers.RouteMappingsRepo{
				Repo: make(map[string]handlers.RouteMapping),
			},
			CAPIDiegoProcessAssociationsRepo: &handlers.CAPIDiegoProcessAssociationsRepo{
				Repo: make(map[handlers.CAPIProcessGUID]handlers.DiegoProcessGUIDs),
			},
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

	Describe("listing Routes (using real repos, to cover more integration-y things)", func() {
		It("returns the routes for each running backend instance", func() {
			handler.RoutesRepo.Upsert(&handlers.Route{
				GUID: "route-guid-a",
				Host: "route-a.cfapps.com",
			})
			routeMapping := handlers.RouteMapping{
				RouteGUID:       "route-guid-a",
				CAPIProcessGUID: "capi-process-guid-a",
			}
			handler.RouteMappingsRepo.Map(routeMapping)
			association := handlers.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-a",
				DiegoProcessGUIDs: handlers.DiegoProcessGUIDs{
					"diego-process-guid-a",
				},
			}
			handler.CAPIDiegoProcessAssociationsRepo.Upsert(association)
			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"diego-process-guid-a.cfapps.internal": backendSetA,
					"diego-process-guid-b.cfapps.internal": backendSetB,
					"route-a.cfapps.com":                   backendSetA,
				},
			}))
		})

		It("ignores route mappings for routes that do not exist", func() {
			routeMapping := handlers.RouteMapping{
				RouteGUID:       "route-guid-a",
				CAPIProcessGUID: "capi-process-guid-a",
			}
			handler.RouteMappingsRepo.Map(routeMapping)
			association := handlers.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-a",
				DiegoProcessGUIDs: handlers.DiegoProcessGUIDs{
					"diego-process-guid-a",
				},
			}
			handler.CAPIDiegoProcessAssociationsRepo.Upsert(association)
			ctx := context.Background()
			resp, err := handler.Routes(ctx, new(api.RoutesRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.RoutesResponse{
				Backends: map[string]*api.BackendSet{
					"diego-process-guid-a.cfapps.internal": backendSetA,
					"diego-process-guid-b.cfapps.internal": backendSetB,
				},
			}))
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
