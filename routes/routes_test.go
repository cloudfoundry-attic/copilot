package routes_test

import (
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/copilot/routes"
	"code.cloudfoundry.org/copilot/routes/fakes"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Collect", func() {
	var (
		rc             *routes.Collector
		logger         *lagertest.TestLogger
		routesRepo     *fakes.RoutesRepo
		routeMappings  *fakes.RouteMappings
		capiDiego      *fakes.CapiDiego
		backendSetRepo *fakes.BackendSet
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		routesRepo = &fakes.RoutesRepo{}
		routeMappings = &fakes.RouteMappings{}
		capiDiego = &fakes.CapiDiego{}
		backendSetRepo = &fakes.BackendSet{}

		rc = routes.NewCollector(
			logger,
			routesRepo,
			routeMappings,
			capiDiego,
			backendSetRepo,
		)
	})

	Context("when an app is running", func() {
		BeforeEach(func() {
			routeMappings.GetCalculatedWeightStub = func(rm *models.RouteMapping) int32 {
				var percent int32
				switch rm.CAPIProcessGUID {
				case "capi-process-guid-a":
					percent = int32(67)
				case "capi-process-guid-c":
					percent = int32(33)
				}

				return percent
			}

			routeMappings.ListReturns(map[string]*models.RouteMapping{
				"route-guid-a-capi-process-guid-a": &models.RouteMapping{
					RouteGUID:       "route-guid-a",
					CAPIProcessGUID: "capi-process-guid-a",
					RouteWeight:     2,
				},
				"route-guid-a-capi-process-guid-c": &models.RouteMapping{
					RouteGUID:       "route-guid-a",
					CAPIProcessGUID: "capi-process-guid-c",
					RouteWeight:     1,
				},
			})

			routesRepo.GetStub = func(guid models.RouteGUID) (*models.Route, bool) {
				r := map[models.RouteGUID]*models.Route{
					"route-guid-a": &models.Route{
						GUID: "route-guid-a",
						Host: "route-a.cfapps.com",
						Path: "",
					},
				}

				return r[guid], true
			}

			capiDiego.GetStub = func(capiProcessGUID *models.CAPIProcessGUID) *models.CAPIDiegoProcessAssociation {
				cd := map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation{
					"capi-process-guid-a": &models.CAPIDiegoProcessAssociation{
						CAPIProcessGUID: "capi-process-guid-a",
						DiegoProcessGUIDs: []models.DiegoProcessGUID{
							"diego-process-guid-a",
						},
					},
					"capi-process-guid-c": &models.CAPIDiegoProcessAssociation{
						CAPIProcessGUID: "capi-process-guid-c",
						DiegoProcessGUIDs: []models.DiegoProcessGUID{
							"diego-process-guid-c",
						},
					},
				}

				return cd[*capiProcessGUID]
			}

			backendSetRepo.GetStub = func(guid models.DiegoProcessGUID) *models.BackendSet {
				bs := map[models.DiegoProcessGUID]*models.BackendSet{
					"diego-process-guid-a": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "10.255.0.16",
								Port:    8080,
							},
							{
								Address: "10.255.1.34",
								Port:    9080,
							},
						},
					},
					"diego-process-guid-c": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "10.255.9.16",
								Port:    8080,
							},
							{
								Address: "10.255.9.34",
								Port:    8080,
							},
						},
					},
				}

				return bs[guid]
			}

		})

		It("returns sorted routes", func() {
			rwb := rc.Collect()
			Expect(rwb).To(HaveLen(2))

			Expect(rwb).To(Equal([]*models.RouteWithBackends{
				&models.RouteWithBackends{
					Hostname: "route-a.cfapps.com",
					Backends: models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "10.255.0.16",
								Port:    8080,
							},
							{
								Address: "10.255.1.34",
								Port:    9080,
							},
						},
					},
					CapiProcessGUID: "capi-process-guid-a",
					RouteWeight:     67,
				},
				&models.RouteWithBackends{
					Hostname: "route-a.cfapps.com",
					Backends: models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "10.255.9.16",
								Port:    8080,
							},
							{
								Address: "10.255.9.34",
								Port:    8080,
							},
						},
					},
					CapiProcessGUID: "capi-process-guid-c",
					RouteWeight:     33,
				},
			}))
		})
	})

	Context("when a route belongs to an internal domain", func() {
		It("skips the route", func() {
			routeMappings.ListReturns(map[string]*models.RouteMapping{
				"route-guid-a-capi-process-guid-a": &models.RouteMapping{
					RouteGUID:       "route-guid-z",
					CAPIProcessGUID: "capi-process-guid-z",
					RouteWeight:     2,
				},
			})

			routesRepo.GetReturns(&models.Route{
				GUID:     "route-guid-z",
				Host:     "look-alive.foo.internal",
				Path:     "",
				Internal: true,
			}, true)

			rc.Collect()

			Expect(routesRepo.GetArgsForCall(0)).To(Equal(models.RouteGUID("route-guid-z")))
			Expect(capiDiego.GetCallCount()).To(Equal(0))
		})

		Context("when the internal domain is apps.internal and internal is false because it is using an older version of CAPI", func() {
			It("skips the route", func() {
				routeMappings.ListReturns(map[string]*models.RouteMapping{
					"route-guid-a-capi-process-guid-a": &models.RouteMapping{
						RouteGUID:       "route-guid-z",
						CAPIProcessGUID: "capi-process-guid-z",
						RouteWeight:     2,
					},
				})

				routesRepo.GetReturns(&models.Route{
					GUID:     "route-guid-z",
					Host:     "look-alive.apps.internal",
					Path:     "",
					Internal: false,
				}, true)

				rc.Collect()

				Expect(routesRepo.GetArgsForCall(0)).To(Equal(models.RouteGUID("route-guid-z")))
				Expect(capiDiego.GetCallCount()).To(Equal(0))
			})
		})
	})

	Context("when a route has no capi process associated", func() {
		It("skips the route", func() {
			routeMappings.ListReturns(map[string]*models.RouteMapping{
				"route-guid-a-capi-process-guid-a": &models.RouteMapping{
					RouteGUID:       "route-guid-z",
					CAPIProcessGUID: "capi-process-guid-z",
					RouteWeight:     2,
				},
			})

			routesRepo.GetReturns(&models.Route{
				GUID: "route-guid-z",
				Host: "test.cfapps.com",
				Path: "/something",
			}, true)

			rc.Collect()

			Expect(routesRepo.GetArgsForCall(0)).To(Equal(models.RouteGUID("route-guid-z")))
			Expect(*capiDiego.GetArgsForCall(0)).To(Equal(models.CAPIProcessGUID("capi-process-guid-z")))
			Expect(backendSetRepo.GetCallCount()).To(Equal(0))
		})
	})
})
