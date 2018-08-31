package handlers_test

import (
	"context"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/handlers/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"code.cloudfoundry.org/copilot/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Capi Handlers", func() {
	var (
		handler                              *handlers.CAPI
		logger                               lager.Logger
		fakeRoutesRepo                       *fakes.RoutesRepo
		fakeCAPIDiegoProcessAssociationsRepo *fakes.CAPIDiegoProcessAssociationsRepo
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeRoutesRepo = &fakes.RoutesRepo{}
		fakeCAPIDiegoProcessAssociationsRepo = &fakes.CAPIDiegoProcessAssociationsRepo{}
		handler = &handlers.CAPI{
			Logger:                           logger,
			RoutesRepo:                       fakeRoutesRepo,
			CAPIDiegoProcessAssociationsRepo: fakeCAPIDiegoProcessAssociationsRepo,
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

	Describe("UpsertRoute", func() {
		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.UpsertRoute(ctx, &api.UpsertRouteRequest{
				Route: &api.Route{
					Guid: "some-route-guid",
				}})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.UpsertRoute(ctx, &api.UpsertRouteRequest{
				Route: &api.Route{
					Host: "some-hostname",
				}})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("adds the route if it is new", func() {
			ctx := context.Background()
			_, err := handler.UpsertRoute(ctx, &api.UpsertRouteRequest{
				Route: &api.Route{
					Guid: "route-guid-a",
					Host: "route-a.example.com",
					Destinations: []*api.Destination{
						{
							CapiProcessGuid: "some-capi-process-guid",
							Weight:          100,
						},
					},
				}})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeRoutesRepo.UpsertCallCount()).To(Equal(1))
			Expect(fakeRoutesRepo.UpsertArgsForCall(0)).To(Equal(&models.Route{
				GUID: "route-guid-a",
				Host: "route-a.example.com",
				Destinations: []*models.Destination{
					{
						CapiProcessGuid: "some-capi-process-guid",
						Weight:          100,
					},
				},
			}))
		})

		It("updates the destinations if they exist", func() {
			ctx := context.Background()
			_, err := handler.UpsertRoute(ctx, &api.UpsertRouteRequest{
				Route: &api.Route{
					Guid: "route-guid-a",
					Host: "route-a.example.com",
					Destinations: []*api.Destination{
						{
							CapiProcessGuid: "some-capi-process-guid",
							Weight:          50,
						},
						{
							CapiProcessGuid: "some-other-capi-process-guid",
							Weight:          50,
						},
					},
				}})
			Expect(err).NotTo(HaveOccurred())

			_, err = handler.UpsertRoute(ctx, &api.UpsertRouteRequest{
				Route: &api.Route{
					Guid: "route-guid-a",
					Host: "route-a.example.com",
					Destinations: []*api.Destination{
						{
							CapiProcessGuid: "some-capi-process-guid",
							Weight:          60,
						},
						{
							CapiProcessGuid: "some-other-capi-process-guid",
							Weight:          40,
						},
					},
				}})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRoutesRepo.UpsertCallCount()).To(Equal(2))

			Expect(fakeRoutesRepo.UpsertArgsForCall(0)).To(Equal(&models.Route{
				GUID: "route-guid-a",
				Host: "route-a.example.com",
				Destinations: []*models.Destination{
					{
						CapiProcessGuid: "some-capi-process-guid",
						Weight:          50,
					},
					{
						CapiProcessGuid: "some-other-capi-process-guid",
						Weight:          50,
					},
				},
			}))

			Expect(fakeRoutesRepo.UpsertArgsForCall(1)).To(Equal(&models.Route{
				GUID: "route-guid-a",
				Host: "route-a.example.com",
				Destinations: []*models.Destination{
					{
						CapiProcessGuid: "some-capi-process-guid",
						Weight:          60,
					},
					{
						CapiProcessGuid: "some-other-capi-process-guid",
						Weight:          40,
					},
				},
			}))
		})

	})

	// Not sure how to handle route deletion yet.
	Describe("DeleteRoute", func() {
		It("calls Delete on the RoutesRepo using the provided guid", func() {
			fakeRoutesRepo := &fakes.RoutesRepo{}
			ctx := context.Background()
			handler.RoutesRepo = fakeRoutesRepo
			_, err := handler.DeleteRoute(ctx, &api.DeleteRouteRequest{Guid: "route-guid-a"})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeRoutesRepo.DeleteCallCount()).To(Equal(1))
			Expect(fakeRoutesRepo.DeleteArgsForCall(0)).To(Equal(models.RouteGUID("route-guid-a")))
		})

		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.DeleteRoute(ctx, &api.DeleteRouteRequest{})
			Expect(err.Error()).To(ContainSubstring("required"))
		})
	})

	Describe("UpsertCapiDiegoProcessAssociation", func() {
		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.UpsertCapiDiegoProcessAssociation(ctx, &api.UpsertCapiDiegoProcessAssociationRequest{})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.UpsertCapiDiegoProcessAssociation(ctx, &api.UpsertCapiDiegoProcessAssociationRequest{
				CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
					CapiProcessGuid: "some-capi-process-guid",
				},
			})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.UpsertCapiDiegoProcessAssociation(ctx, &api.UpsertCapiDiegoProcessAssociationRequest{
				CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
					DiegoProcessGuids: []string{
						"some-diego-process-guid",
					},
				},
			})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("associates the capi and diego process guids", func() {
			ctx := context.Background()
			_, err := handler.UpsertCapiDiegoProcessAssociation(ctx, &api.UpsertCapiDiegoProcessAssociationRequest{
				CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
					CapiProcessGuid: "some-capi-process-guid",
					DiegoProcessGuids: []string{
						"some-diego-process-guid-1",
						"some-diego-process-guid-2",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCAPIDiegoProcessAssociationsRepo.UpsertCallCount()).To(Equal(1))
			Expect(fakeCAPIDiegoProcessAssociationsRepo.UpsertArgsForCall(0)).To(Equal(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "some-capi-process-guid",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				},
			}))
		})
	})

	Describe("DeleteCapiDiegoProcessAssociation", func() {
		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.DeleteCapiDiegoProcessAssociation(ctx, &api.DeleteCapiDiegoProcessAssociationRequest{})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("deletes the association", func() {
			ctx := context.Background()
			_, err := handler.DeleteCapiDiegoProcessAssociation(ctx, &api.DeleteCapiDiegoProcessAssociationRequest{
				CapiProcessGuid: "some-capi-process-guid",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCAPIDiegoProcessAssociationsRepo.DeleteCallCount()).To(Equal(1))
			cpg := models.CAPIProcessGUID("some-capi-process-guid")
			Expect(fakeCAPIDiegoProcessAssociationsRepo.DeleteArgsForCall(0)).To(Equal(&cpg))
		})
	})

	Describe("BulkSync", func() {
		Context("when inputs are empty", func() {
			It("does not throw an error", func() {
				ctx := context.Background()
				_, err := handler.BulkSync(ctx, &api.BulkSyncRequest{})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeRoutesRepo.SyncCallCount()).To(Equal(1))
				Expect(fakeCAPIDiegoProcessAssociationsRepo.SyncCallCount()).To(Equal(1))
			})
		})

		It("syncs", func() {
			ctx := context.Background()
			_, err := handler.BulkSync(ctx, &api.BulkSyncRequest{
				Routes: []*api.Route{
					{
						Guid: "route-guid-a",
						Host: "example.host.com",
						Path: "/nothing/matters",
						Destinations: []*api.Destination{
							{
								CapiProcessGuid: "some-capi-process-guid",
								Weight:          100,
							},
						},
					},
				},
				CapiDiegoProcessAssociations: []*api.CapiDiegoProcessAssociation{{
					CapiProcessGuid: "some-capi-process-guid",
					DiegoProcessGuids: []string{
						"some-diego-process-guid-1",
						"some-diego-process-guid-2",
					},
				}},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRoutesRepo.SyncCallCount()).To(Equal(1))
			Expect(fakeRoutesRepo.SyncArgsForCall(0)).To(Equal([]*models.Route{
				{
					GUID: "route-guid-a",
					Host: "example.host.com",
					Path: "/nothing/matters",
					Destinations: []*models.Destination{
						{
							CapiProcessGuid: "some-capi-process-guid",
							Weight:          100,
						},
					},
				},
			}))

			Expect(fakeCAPIDiegoProcessAssociationsRepo.SyncCallCount()).To(Equal(1))
			Expect(fakeCAPIDiegoProcessAssociationsRepo.SyncArgsForCall(0)).To(Equal([]*models.CAPIDiegoProcessAssociation{{
				CAPIProcessGUID: "some-capi-process-guid",
				DiegoProcessGUIDs: []models.DiegoProcessGUID{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				},
			}}))
		})
	})
})
