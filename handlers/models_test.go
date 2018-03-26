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
			routesRepo = handlers.RoutesRepo{
				Repo: make(map[handlers.RouteGUID]*handlers.Route),
			}
		})

		It("can Upsert and Delete routes", func() {
			route := &handlers.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
			}

			go routesRepo.Upsert(route)

			Eventually(func() *handlers.Route {
				r, _ := routesRepo.Get("some-route-guid")
				return r
			}).Should(Equal(route))

			routesRepo.Delete(route.GUID)

			r, ok := routesRepo.Get("some-route-guid")
			Expect(ok).To(BeFalse())
			Expect(r).To(BeNil())
		})

		It("does not error when deleting a route that does not exist", func() {
			route := handlers.Route{
				Host: "host.example.com",
				GUID: "delete-me",
			}

			routesRepo.Delete(route.GUID)
			routesRepo.Delete(route.GUID)

			_, ok := routesRepo.Get("delete-me")
			Expect(ok).To(BeFalse())
		})

		It("can Upsert the same route twice", func() {
			route := &handlers.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
			}

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
			routeMappingsRepo = handlers.RouteMappingsRepo{
				Repo: make(map[string]handlers.RouteMapping),
			}
		})

		It("can Map and Unmap Routes", func() {
			routeMapping := handlers.RouteMapping{
				RouteGUID:       "some-route-guid",
				CAPIProcessGUID: "some-capi-guid",
			}

			go routeMappingsRepo.Map(routeMapping)

			Eventually(routeMappingsRepo.List).Should(Equal(map[string]handlers.RouteMapping{
				routeMapping.Key(): routeMapping,
			}))

			routeMappingsRepo.Unmap(routeMapping)
			Expect(routeMappingsRepo.List()).To(HaveLen(0))
		})

		It("does not duplicate route mappings", func() {
			routeMapping := handlers.RouteMapping{
				RouteGUID:       "some-route-guid",
				CAPIProcessGUID: "some-capi-guid",
			}

			routeMappingsRepo.Map(routeMapping)
			routeMappingsRepo.Map(routeMapping)
			routeMappingsRepo.Map(routeMapping)

			Expect(routeMappingsRepo.List()).To(HaveLen(1))
		})
	})

	Describe("CAPIDiegoProcessAssociationsRepo", func() {
		var capiDiegoProcessAssociationsRepo handlers.CAPIDiegoProcessAssociationsRepo
		BeforeEach(func() {
			capiDiegoProcessAssociationsRepo = handlers.CAPIDiegoProcessAssociationsRepo{
				Repo: make(map[handlers.CAPIProcessGUID]handlers.CAPIDiegoProcessAssociation),
			}
		})

		It("can upsert and delete CAPIDiegoProcessAssociations", func() {
			capiDiegoProcessAssociation := handlers.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "some-capi-process-guid",
				DiegoProcessGUIDs: handlers.DiegoProcessGUIDs{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				},
			}

			go capiDiegoProcessAssociationsRepo.Upsert(capiDiegoProcessAssociation)

			Eventually(func() handlers.CAPIDiegoProcessAssociation {
				return capiDiegoProcessAssociationsRepo.Get("some-capi-process-guid")
			}).Should(Equal(capiDiegoProcessAssociation))

			capiDiegoProcessAssociationsRepo.Delete(capiDiegoProcessAssociation.CAPIProcessGUID)
			Expect(capiDiegoProcessAssociationsRepo.Get("some-capi-process-guid").DiegoProcessGUIDs).To(BeEmpty())
		})
	})

	Describe("RouteMapping", func() {
		Describe("Key", func() {
			It("is unique for process guid and route guid", func() {
				rmA := handlers.RouteMapping{
					RouteGUID:       "route-guid-1",
					CAPIProcessGUID: "some-capi-guid-1",
				}

				rmB := handlers.RouteMapping{
					RouteGUID:       "route-guid-1",
					CAPIProcessGUID: "some-capi-guid-2",
				}

				rmC := handlers.RouteMapping{
					RouteGUID:       "route-guid-2",
					CAPIProcessGUID: "some-capi-guid-1",
				}

				Expect(rmA.Key()).NotTo(Equal(rmB.Key()))
				Expect(rmA.Key()).NotTo(Equal(rmC.Key()))
				Expect(rmB.Key()).NotTo(Equal(rmC.Key()))
			})
		})
	})

	Describe("DiegoProcessGUIDs", func() {
		Describe("ToStringSlice", func() {
			It("returns the guids as a slice of strings", func() {
				diegoProcessGUIDs := handlers.DiegoProcessGUIDs{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				}
				Expect(diegoProcessGUIDs.ToStringSlice()).To(Equal([]string{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				}))
			})
		})
	})

	Describe("DiegoProcessGUIDsFromStringSlice", func() {
		It("returns the guids as DiegoProcessGUIDs", func() {
			diegoProcessGUIDs := []string{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			}
			Expect(handlers.DiegoProcessGUIDsFromStringSlice(diegoProcessGUIDs)).To(Equal(handlers.DiegoProcessGUIDs{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			}))
		})
	})
})
