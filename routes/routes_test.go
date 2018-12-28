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
		vipProvider    *fakes.VIPProvider
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		routesRepo = &fakes.RoutesRepo{}
		routeMappings = &fakes.RouteMappings{}
		capiDiego = &fakes.CapiDiego{}
		backendSetRepo = &fakes.BackendSet{}
		vipProvider = &fakes.VIPProvider{}

		rc = routes.NewCollector(
			logger,
			routesRepo,
			routeMappings,
			capiDiego,
			backendSetRepo,
			vipProvider,
		)
	})

	Context("when number of apps don't divide evenly into 100", func() {
		BeforeEach(func() {
			routeMappings.GetCalculatedWeightStub = func(rm *models.RouteMapping) int32 {
				var percent int32
				switch rm.CAPIProcessGUID {
				case "capi-process-guid-a":
					percent = int32(33)
				case "capi-process-guid-b":
					percent = int32(33)
				case "capi-process-guid-c":
					percent = int32(33)
				case "capi-process-guid-d":
					percent = int32(100)
				}

				return percent
			}

			routeMappings.ListReturns(map[string]*models.RouteMapping{
				"route-guid-a-capi-process-guid-a": &models.RouteMapping{
					RouteGUID:       "route-guid-a",
					CAPIProcessGUID: "capi-process-guid-a",
					RouteWeight:     1,
				},
				"route-guid-a-capi-process-guid-b": &models.RouteMapping{
					RouteGUID:       "route-guid-a",
					CAPIProcessGUID: "capi-process-guid-b",
					RouteWeight:     1,
				},
				"route-guid-a-capi-process-guid-c": &models.RouteMapping{
					RouteGUID:       "route-guid-a",
					CAPIProcessGUID: "capi-process-guid-c",
					RouteWeight:     1,
				},
				"route-guid-b-capi-process-guid-d": &models.RouteMapping{
					RouteGUID:       "route-guid-b",
					CAPIProcessGUID: "capi-process-guid-d",
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
					"route-guid-b": &models.Route{
						GUID: "route-guid-b",
						Host: "route-b.cfapps.com",
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
					"capi-process-guid-b": &models.CAPIDiegoProcessAssociation{
						CAPIProcessGUID: "capi-process-guid-b",
						DiegoProcessGUIDs: []models.DiegoProcessGUID{
							"diego-process-guid-b",
						},
					},
					"capi-process-guid-c": &models.CAPIDiegoProcessAssociation{
						CAPIProcessGUID: "capi-process-guid-c",
						DiegoProcessGUIDs: []models.DiegoProcessGUID{
							"diego-process-guid-c",
						},
					},
					"capi-process-guid-d": &models.CAPIDiegoProcessAssociation{
						CAPIProcessGUID: "capi-process-guid-d",
						DiegoProcessGUIDs: []models.DiegoProcessGUID{
							"diego-process-guid-d",
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
								Address: "1.1.1.1",
								Port:    1111,
							},
						},
					},
					"diego-process-guid-b": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "2.2.2.2",
								Port:    2222,
							},
						},
					},
					"diego-process-guid-c": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "3.3.3.3",
								Port:    3333,
							},
						},
					},
					"diego-process-guid-d": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "4.4.4.4",
								Port:    4444,
							},
						},
					},
				}

				return bs[guid]
			}
		})

		It("fixes the weights so the sum is 100", func() {
			rwb := rc.Collect()
			Expect(rwb).To(HaveLen(4))

			Expect(rwb).To(Equal([]*models.RouteWithBackends{
				&models.RouteWithBackends{
					Hostname: "route-a.cfapps.com",
					Backends: models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "1.1.1.1",
								Port:    1111,
							},
						},
					},
					CapiProcessGUID: "capi-process-guid-a",
					RouteWeight:     33,
				},
				&models.RouteWithBackends{
					Hostname: "route-a.cfapps.com",
					Backends: models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "2.2.2.2",
								Port:    2222,
							},
						},
					},
					CapiProcessGUID: "capi-process-guid-b",
					RouteWeight:     33,
				},
				&models.RouteWithBackends{
					Hostname: "route-a.cfapps.com",
					Backends: models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "3.3.3.3",
								Port:    3333,
							},
						},
					},
					CapiProcessGUID: "capi-process-guid-c",
					RouteWeight:     34,
				},
				&models.RouteWithBackends{
					Hostname: "route-b.cfapps.com",
					Backends: models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "4.4.4.4",
								Port:    4444,
							},
						},
					},
					CapiProcessGUID: "capi-process-guid-d",
					RouteWeight:     100,
				},
			}))
		})
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
								Address: "10.0.1.16",
								Port:    61080,
							},
							{
								Address: "10.0.2.34",
								Port:    61090,
							},
						},
					},
					"diego-process-guid-c": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "10.0.9.16",
								Port:    61080,
							},
							{
								Address: "10.0.9.34",
								Port:    61080,
							},
						},
					},
				}

				return bs[guid]
			}

			backendSetRepo.GetInternalBackendsStub = func(guid models.DiegoProcessGUID) *models.BackendSet {
				bs := map[models.DiegoProcessGUID]*models.BackendSet{
					"diego-process-guid-a": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "10.255.2.2",
								Port:    8080,
							},
							{
								Address: "10.255.3.3",
								Port:    9090,
							},
						},
					},
					"diego-process-guid-c": &models.BackendSet{
						Backends: []*models.Backend{
							{
								Address: "10.255.8.8",
								Port:    8080,
							},
							{
								Address: "10.255.9.9",
								Port:    9090,
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
								Address: "10.0.1.16",
								Port:    61080,
							},
							{
								Address: "10.0.2.34",
								Port:    61090,
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
								Address: "10.0.9.16",
								Port:    61080,
							},
							{
								Address: "10.0.9.34",
								Port:    61080,
							},
						},
					},
					CapiProcessGUID: "capi-process-guid-c",
					RouteWeight:     33,
				},
			}))
		})

		Context("when a route belongs to an internal domain", func() {
			BeforeEach(func() {
				routesRepo.GetStub = func(guid models.RouteGUID) (*models.Route, bool) {
					r := map[models.RouteGUID]*models.Route{
						"route-guid-a": &models.Route{
							GUID:     "route-guid-a",
							Host:     "route-a.foo.internal",
							Path:     "",
							Internal: true,
						},
					}

					return r[guid], true
				}

				vipProvider.GetReturns("127.127.1.1")
			})

			It("marks the route with backend as internal", func() {
				rwb := rc.Collect()

				Expect(vipProvider.GetCallCount()).To(Equal(2))
				Expect(vipProvider.GetArgsForCall(0)).To(Equal("route-a.foo.internal"))

				Expect(rwb).To(HaveLen(2))

				Expect(rwb).To(Equal([]*models.RouteWithBackends{
					&models.RouteWithBackends{
						Hostname: "route-a.foo.internal",
						Internal: true,
						VIP:      "127.127.1.1",
						Backends: models.BackendSet{
							Backends: []*models.Backend{
								{
									Address: "10.255.2.2",
									Port:    8080,
								},
								{
									Address: "10.255.3.3",
									Port:    9090,
								},
							},
						},
						CapiProcessGUID: "capi-process-guid-a",
						RouteWeight:     67,
					},
					&models.RouteWithBackends{
						Hostname: "route-a.foo.internal",
						Internal: true,
						VIP:      "127.127.1.1",
						Backends: models.BackendSet{
							Backends: []*models.Backend{
								{
									Address: "10.255.8.8",
									Port:    8080,
								},
								{
									Address: "10.255.9.9",
									Port:    9090,
								},
							},
						},
						CapiProcessGUID: "capi-process-guid-c",
						RouteWeight:     33,
					},
				}))
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

	Context("when a route has an empty backend set", func() {
		It("skips the route", func() {
			routeMappings.ListReturns(map[string]*models.RouteMapping{
				"route-guid-a-capi-process-guid-a": &models.RouteMapping{
					RouteGUID:       "route-guid-z",
					CAPIProcessGUID: "capi-process-guid-z",
					RouteWeight:     2,
				},
			})

			capiDiego.GetStub = func(capiProcessGUID *models.CAPIProcessGUID) *models.CAPIDiegoProcessAssociation {
				cd := map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation{
					"capi-process-guid-z": &models.CAPIDiegoProcessAssociation{
						CAPIProcessGUID: "capi-process-guid-z",
						DiegoProcessGUIDs: []models.DiegoProcessGUID{
							"diego-process-guid-z",
						},
					},
				}

				return cd[*capiProcessGUID]
			}

			routesRepo.GetReturns(&models.Route{
				GUID: "route-guid-z",
				Host: "test.cfapps.com",
				Path: "/something",
			}, true)

			rwb := rc.Collect()

			Expect(routesRepo.GetArgsForCall(0)).To(Equal(models.RouteGUID("route-guid-z")))
			Expect(*capiDiego.GetArgsForCall(0)).To(Equal(models.CAPIProcessGUID("capi-process-guid-z")))
			Expect(backendSetRepo.GetCallCount()).To(Equal(1))
			Expect(rwb).To(HaveLen(0))
		})
	})
})
