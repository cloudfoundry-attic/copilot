package models_test

import (
	"code.cloudfoundry.org/copilot/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RouteMappingsRepo", func() {
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
})
