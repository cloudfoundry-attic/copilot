package models_test

import (
	"code.cloudfoundry.org/copilot/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutesRepo", func() {
	var routesRepo *models.RoutesRepo

	BeforeEach(func() {
		routesRepo = models.NewRoutesRepo()
	})

	// We don't know how to delete routes yet.
	Describe("Delete", func() {
		It("deletes upsert route", func() {
			route := &models.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
			}

			go routesRepo.Upsert(route)

			Eventually(func() *models.Route {
				r, _ := routesRepo.Get("some-route-guid")
				return r
			}).Should(Equal(route))

			routesRepo.Delete(route.GUID)

			r, ok := routesRepo.Get("some-route-guid")
			Expect(ok).To(BeFalse())
			Expect(r).To(BeNil())
		})

		Context("when deleting a route that does not exist", func() {
			It("does not return an error", func() {
				route := models.Route{
					Host: "host.example.com",
					GUID: "delete-me",
				}

				routesRepo.Delete(route.GUID)
				routesRepo.Delete(route.GUID)

				_, ok := routesRepo.Get("delete-me")
				Expect(ok).To(BeFalse())
			})
		})
	})

	Describe("Upsert", func() {
		It("updates the same route", func() {
			route := &models.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
				Destinations: []*models.Destination{
					{
						CAPIProcessGUID: "some-capi-process-guid",
						Weight:          60,
					},
					{
						CAPIProcessGUID: "some-other-capi-process-guid",
						Weight:          40,
					},
				},
			}

			updatedRoute := &models.Route{
				Host: "something.different.com",
				GUID: route.GUID,
				Destinations: []*models.Destination{
					{
						CAPIProcessGUID: "some-capi-process-guid",
						Weight:          50,
					},
					{
						CAPIProcessGUID: "some-other-capi-process-guid",
						Weight:          50,
					},
				},
			}

			routesRepo.Upsert(updatedRoute)

			r, _ := routesRepo.Get("some-route-guid")
			Expect(r).To(Equal(updatedRoute))
		})

		It("downcases hosts", func() {
			route := &models.Route{
				Host: "HOST.example.com",
				GUID: "some-route-guid",
			}

			routesRepo.Upsert(route)
			r, _ := routesRepo.Get("some-route-guid")
			Expect(r.Hostname()).To(Equal("host.example.com"))
		})
	})

	Describe("Sync", func() {
		It("saves routes", func() {
			route := &models.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
				Destinations: []*models.Destination{
					{
						CAPIProcessGUID: "some-capi-process-guid",
						Weight:          100,
					},
				},
			}

			go routesRepo.Upsert(route)

			Eventually(func() *models.Route {
				r, _ := routesRepo.Get("some-route-guid")
				return r
			}).Should(Equal(route))

			newRoute := &models.Route{
				Host: "host.example.com",
				GUID: "some-other-route-guid",
				Destinations: []*models.Destination{
					{
						CAPIProcessGUID: "some-capi-process-guid",
						Weight:          100,
					},
				},
			}

			routesRepo.Sync([]*models.Route{newRoute})
			Expect(routesRepo.List()).To(Equal(map[string]*models.Route{
				string(newRoute.GUID): newRoute,
			}))
		})
	})

	Describe("List", func() {
		It("returns all routes", func() {
			route := &models.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
				Destinations: []*models.Destination{
					{
						CAPIProcessGUID: "some-capi-process-guid",
						Weight:          60,
					},
					{
						CAPIProcessGUID: "some-other-capi-process-guid",
						Weight:          40,
					},
				},
			}

			route2 := &models.Route{
				Host: "something.different.com",
				GUID: "some-other-route-guid",
				Destinations: []*models.Destination{
					{
						CAPIProcessGUID: "some-capi-process-guid",
						Weight:          50,
					},
					{
						CAPIProcessGUID: "some-other-capi-process-guid",
						Weight:          50,
					},
				},
			}

			routesRepo.Upsert(route)
			routesRepo.Upsert(route2)

			routes := routesRepo.List()
			Expect(len(routes)).To(Equal(2))
		})
	})
})
