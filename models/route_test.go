package models_test

import (
	"code.cloudfoundry.org/copilot/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutesRepo", func() {
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

	It("downcases hosts", func() {
		route := &models.Route{
			Host: "HOST.example.com",
			GUID: "some-route-guid",
		}

		routesRepo.Upsert(route)
		r, _ := routesRepo.Get("some-route-guid")
		Expect(r.Hostname()).To(Equal("host.example.com"))
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
