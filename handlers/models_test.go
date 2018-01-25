package handlers_test

import (
	"code.cloudfoundry.org/copilot/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler Models", func() {
	Describe("RoutesRepo", func() {
		var routesRepo handlers.RoutesRepo
		BeforeEach(func() {
			routesRepo = handlers.RoutesRepo{}
		})
		It("Can Upsert and Delete Routes", func() {
			route := &handlers.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
			}
			routesRepo.Upsert(route)

			r, _ := routesRepo.Get("some-route-guid")
			Expect(r).To(Equal(route))
			routesRepo.Delete(route.GUID)
			r, ok := routesRepo.Get("some-route-guid")
			Expect(ok).To(BeFalse())
			Expect(r).To(BeNil())
		})

		It("Does not error when deleting a route that does not exist", func() {
			route := &handlers.Route{
				Host: "host.example.com",
				GUID: "delete-me",
			}
			routesRepo.Delete(route.GUID)
			routesRepo.Delete(route.GUID)

			_, ok := routesRepo.Get("delete-me")
			Expect(ok).To(BeFalse())
		})

		It("Can Upsert the same route twice", func() {
			route := &handlers.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
			}
			routesRepo.Upsert(route)
			updatedRoute := &handlers.Route{
				Host: "something.different.com",
				GUID: route.GUID,
			}
			routesRepo.Upsert(updatedRoute)

			r, _ := routesRepo.Get("some-route-guid")
			Expect(r).To(Equal(updatedRoute))
		})
	})

	Describe("RouteMappingsRepo", func() {
		var routeMappingsRepo handlers.RouteMappingsRepo
		BeforeEach(func() {
			routeMappingsRepo = handlers.RouteMappingsRepo{}
		})

		It("Can Map and Unmap Routes", func() {
			routeMapping := &handlers.RouteMapping{
				RouteGUID: "some-route-guid",
				CAPIProcess: &handlers.CAPIProcess{
					GUID: "some-capi-guid",
					DiegoProcessGUID: handlers.DiegoProcessGUID("some-process-guid"),
				},
			}
			routeMappingsRepo.Map(routeMapping)
			Expect(routeMappingsRepo.List()).To(Equal(map[string]*handlers.RouteMapping{
				routeMapping.Key(): routeMapping,
			}))
			routeMappingsRepo.Unmap(routeMapping)
			Expect(routeMappingsRepo.List()).To(HaveLen(0))
		})

		It("does not duplicate route mappings", func() {
			routeMapping := &handlers.RouteMapping{
				RouteGUID: "some-route-guid",
				CAPIProcess: &handlers.CAPIProcess{
					GUID: "some-capi-guid",
					DiegoProcessGUID: handlers.DiegoProcessGUID("some-process-guid"),
				},
			}
			routeMappingsRepo.Map(routeMapping)
			routeMappingsRepo.Map(routeMapping)
			routeMappingsRepo.Map(routeMapping)
			Expect(routeMappingsRepo.List()).To(HaveLen(1))
		})
	})

	Describe("RouteMapping", func() {
		Describe("Key", func() {
			It("Is unique for process guid and route guid", func() {
				rmA := handlers.RouteMapping{
					RouteGUID: "route-guid-1",
					CAPIProcess: &handlers.CAPIProcess{
						GUID: "some-capi-guid-1",
						DiegoProcessGUID: handlers.DiegoProcessGUID("process-guid-1"),
					},
				}
				rmB := handlers.RouteMapping{
					RouteGUID: "route-guid-1",
					CAPIProcess: &handlers.CAPIProcess{
						GUID: "some-capi-guid-2",
						DiegoProcessGUID: handlers.DiegoProcessGUID("process-guid-2"),
					},
				}
				rmC := handlers.RouteMapping{
					RouteGUID: "route-guid-2",
					CAPIProcess: &handlers.CAPIProcess{
						GUID: "some-capi-guid-1",
						DiegoProcessGUID: handlers.DiegoProcessGUID("process-guid-1"),
					},
				}
				Expect(rmA.Key()).NotTo(Equal(rmB.Key()))
				Expect(rmA.Key()).NotTo(Equal(rmC.Key()))
				Expect(rmB.Key()).NotTo(Equal(rmC.Key()))
			})
		})
	})
})