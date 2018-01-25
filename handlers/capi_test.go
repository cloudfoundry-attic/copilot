package handlers_test

import (
	"context"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/handlers/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Capi Handlers", func() {
	var (
		handler               *handlers.CAPI
		logger                lager.Logger
		fakeRoutesRepo        *fakes.RoutesRepo
		fakeRouteMappingsRepo *fakes.RouteMappingsRepo
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeRoutesRepo = &fakes.RoutesRepo{}
		fakeRouteMappingsRepo = &fakes.RouteMappingsRepo{}
		handler = &handlers.CAPI{
			Logger:            logger,
			RoutesRepo:        fakeRoutesRepo,
			RouteMappingsRepo: fakeRouteMappingsRepo,
		}
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
				}})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeRoutesRepo.UpsertCallCount()).To(Equal(1))
			Expect(fakeRoutesRepo.UpsertArgsForCall(0)).To(Equal(&handlers.Route{
				GUID: "route-guid-a",
				Host: "route-a.example.com",
			}))
		})
	})

	Describe("DeleteRoute", func() {
		It("calls Delete on the RoutesRepo using the provided guid", func() {
			fakeRoutesRepo := &fakes.RoutesRepo{}
			ctx := context.Background()
			handler.RoutesRepo = fakeRoutesRepo
			_, err := handler.DeleteRoute(ctx, &api.DeleteRouteRequest{Guid: "route-guid-a"})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeRoutesRepo.DeleteCallCount()).To(Equal(1))
			Expect(fakeRoutesRepo.DeleteArgsForCall(0)).To(Equal(handlers.RouteGUID("route-guid-a")))
		})

		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.DeleteRoute(ctx, &api.DeleteRouteRequest{})
			Expect(err.Error()).To(ContainSubstring("required"))
		})
	})

	Describe("MapRoute", func() {
		BeforeEach(func() {
			handler.RoutesRepo.Upsert(&handlers.Route{
				GUID: "route-guid-a",
				Host: "route-a.example.com",
			})
		})

		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.MapRoute(ctx, &api.MapRouteRequest{RouteMapping: &api.RouteMapping{
				RouteGuid: "some-route-guid",
			}})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.MapRoute(ctx, &api.MapRouteRequest{RouteMapping: &api.RouteMapping{
				CapiProcess: &api.CapiProcess{Guid: "some-process-guid"},
			}})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("maps the route", func() {
			ctx := context.Background()
			_, err := handler.MapRoute(ctx, &api.MapRouteRequest{
				RouteMapping: &api.RouteMapping{
					RouteGuid: "route-guid-a",
					CapiProcess: &api.CapiProcess{
						Guid:             "some-capi-process-guid",
						DiegoProcessGuid: "process-guid-a",
					},
				}})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeRouteMappingsRepo.MapCallCount()).To(Equal(1))
			Expect(fakeRouteMappingsRepo.MapArgsForCall(0)).To(Equal(handlers.RouteMapping{
				RouteGUID: "route-guid-a",
				CAPIProcess: &handlers.CAPIProcess{
					GUID:             "some-capi-process-guid",
					DiegoProcessGUID: "process-guid-a",
				},
			}))
		})
	})

	Describe("UnmapRoute", func() {
		It("validates the inputs", func() {
			ctx := context.Background()
			_, err := handler.UnmapRoute(ctx, &api.UnmapRouteRequest{RouteGuid: "some-route-guid"})
			Expect(err.Error()).To(ContainSubstring("required"))
			_, err = handler.UnmapRoute(ctx, &api.UnmapRouteRequest{CapiProcessGuid: "some-process-guid"})
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("unmaps the routes", func() {
			ctx := context.Background()
			_, err := handler.UnmapRoute(ctx, &api.UnmapRouteRequest{RouteGuid: "to-be-deleted-route-guid", CapiProcessGuid: "some-capi-process-guid"})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeRouteMappingsRepo.UnmapCallCount()).To(Equal(1))
			Expect(fakeRouteMappingsRepo.UnmapArgsForCall(0)).To(Equal(handlers.RouteMapping{
				RouteGUID: "to-be-deleted-route-guid",
				CAPIProcess: &handlers.CAPIProcess{
					GUID: "some-capi-process-guid",
				},
			}))
		})
	})
})
