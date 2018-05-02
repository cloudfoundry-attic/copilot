package models_test

import (
	"code.cloudfoundry.org/copilot/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler Models", func() {
	Describe("RoutesRepo", func() {
		var routesRepo models.RoutesRepo

		BeforeEach(func() {
			routesRepo = models.RoutesRepo{
				Repo: make(map[models.RouteGUID]*models.Route),
			}
		})

		It("can Upsert and Delete routes", func() {
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

		It("does not error when deleting a route that does not exist", func() {
			route := models.Route{
				Host: "host.example.com",
				GUID: "delete-me",
			}

			routesRepo.Delete(route.GUID)
			routesRepo.Delete(route.GUID)

			_, ok := routesRepo.Get("delete-me")
			Expect(ok).To(BeFalse())
		})

		It("can Upsert the same route twice", func() {
			route := &models.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
			}

			updatedRoute := &models.Route{
				Host: "something.different.com",
				GUID: route.GUID,
			}

			routesRepo.Upsert(updatedRoute)

			r, _ := routesRepo.Get("some-route-guid")
			Expect(r).To(Equal(updatedRoute))
		})

		It("can Sync routes", func() {
			route := &models.Route{
				Host: "host.example.com",
				GUID: "some-route-guid",
			}

			go routesRepo.Upsert(route)

			Eventually(func() *models.Route {
				r, _ := routesRepo.Get("some-route-guid")
				return r
			}).Should(Equal(route))

			newRoute := &models.Route{
				Host: "host.example.com",
				GUID: "some-other-route-guid",
			}

			routesRepo.Sync([]*models.Route{newRoute})
			Expect(routesRepo.List()).To(Equal(map[string]string{
				string(newRoute.GUID): newRoute.Host,
			}))
		})
	})

	Describe("RouteMappingsRepo", func() {
		var routeMappingsRepo *models.RouteMappingsRepo
		BeforeEach(func() {
			routeMappingsRepo = &models.RouteMappingsRepo{
				Repo: make(map[string]*models.RouteMapping),
			}
		})

		It("can Map and Unmap Routes", func() {
			routeMapping := models.RouteMapping{
				RouteGUID:       "some-route-guid",
				CAPIProcessGUID: "some-capi-guid",
			}

			go routeMappingsRepo.Map(&routeMapping)

			Eventually(routeMappingsRepo.List).Should(Equal(map[string]*models.RouteMapping{
				routeMapping.Key(): &routeMapping,
			}))

			routeMappingsRepo.Unmap(&routeMapping)
			Expect(routeMappingsRepo.List()).To(HaveLen(0))
		})

		It("does not duplicate route mappings", func() {
			routeMapping := models.RouteMapping{
				RouteGUID:       "some-route-guid",
				CAPIProcessGUID: "some-capi-guid",
			}

			routeMappingsRepo.Map(&routeMapping)
			routeMappingsRepo.Map(&routeMapping)
			routeMappingsRepo.Map(&routeMapping)

			Expect(routeMappingsRepo.List()).To(HaveLen(1))
		})

		It("can Sync RouteMappings", func() {
			routeMapping := models.RouteMapping{
				RouteGUID:       "some-route-guid",
				CAPIProcessGUID: "some-capi-guid",
			}

			routeMappingsRepo.Map(&routeMapping)

			newRouteMapping := models.RouteMapping{
				RouteGUID:       "some-other-route-guid",
				CAPIProcessGUID: "some-other-capi-guid",
			}
			updatedRouteMappings := []*models.RouteMapping{&newRouteMapping}

			routeMappingsRepo.Sync(updatedRouteMappings)

			Eventually(routeMappingsRepo.List).Should(Equal(map[string]*models.RouteMapping{
				newRouteMapping.Key(): &newRouteMapping,
			}))
		})
	})

	Describe("CAPIDiegoProcessAssociationsRepo", func() {
		var capiDiegoProcessAssociationsRepo models.CAPIDiegoProcessAssociationsRepo
		BeforeEach(func() {
			capiDiegoProcessAssociationsRepo = models.CAPIDiegoProcessAssociationsRepo{
				Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
			}
		})

		It("can upsert and delete CAPIDiegoProcessAssociations", func() {
			capiDiegoProcessAssociation := models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "some-capi-process-guid",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				},
			}

			go capiDiegoProcessAssociationsRepo.Upsert(&capiDiegoProcessAssociation)

			capiProcessGUID := models.CAPIProcessGUID("some-capi-process-guid")

			Eventually(func() *models.CAPIDiegoProcessAssociation {
				return capiDiegoProcessAssociationsRepo.Get(&capiProcessGUID)
			}).Should(Equal(&capiDiegoProcessAssociation))

			capiDiegoProcessAssociationsRepo.Delete(&capiDiegoProcessAssociation.CAPIProcessGUID)
			Expect(capiDiegoProcessAssociationsRepo.Get(&capiProcessGUID)).To(BeNil())
		})

		It("can sync CAPIDiegoProcessAssociations", func() {
			capiDiegoProcessAssociation := &models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "some-capi-process-guid",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				},
			}

			capiDiegoProcessAssociationsRepo.Upsert(capiDiegoProcessAssociation)

			newCapiDiegoProcessAssociation := &models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "some-other-capi-process-guid",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"some-diego-process-guid-1",
					"some-diego-process-guid-2",
				},
			}

			capiDiegoProcessAssociationsRepo.Sync([]*models.CAPIDiegoProcessAssociation{newCapiDiegoProcessAssociation})

			Expect(capiDiegoProcessAssociationsRepo.List()).To(Equal(map[models.CAPIProcessGUID]*models.DiegoProcessGUIDs{
				"some-other-capi-process-guid": &newCapiDiegoProcessAssociation.DiegoProcessGUIDs,
			}))
		})
	})

	Describe("RouteMapping", func() {
		Describe("Key", func() {
			It("is unique for process guid and route guid", func() {
				rmA := models.RouteMapping{
					RouteGUID:       "route-guid-1",
					CAPIProcessGUID: "some-capi-guid-1",
				}

				rmB := models.RouteMapping{
					RouteGUID:       "route-guid-1",
					CAPIProcessGUID: "some-capi-guid-2",
				}

				rmC := models.RouteMapping{
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
				diegoProcessGUIDs := models.DiegoProcessGUIDs{
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
			Expect(models.DiegoProcessGUIDsFromStringSlice(diegoProcessGUIDs)).To(Equal(models.DiegoProcessGUIDs{
				"some-diego-process-guid-1",
				"some-diego-process-guid-2",
			}))
		})
	})
})
