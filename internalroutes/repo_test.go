package internalroutes_test

import (
	"code.cloudfoundry.org/copilot/internalroutes"
	"code.cloudfoundry.org/copilot/internalroutes/fakes"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repo", func() {
	Describe("Get", func() {
		var (
			routesRepo                       *models.RoutesRepo
			routeMappingsRepo                *models.RouteMappingsRepo
			capiDiegoProcessAssociationsRepo *models.CAPIDiegoProcessAssociationsRepo
			backendSetRepo                   *fakes.BackendSetRepo
			logger                           lager.Logger
			vipProvider                      *fakes.VIPProvider
			internalRoutesRepo               *internalroutes.Repo
		)

		BeforeEach(func() {
			backendSetRepo = &fakes.BackendSetRepo{}
			logger = lagertest.NewTestLogger("test")

			routesRepo = models.NewRoutesRepo()
			routeMappingsRepo = models.NewRouteMappingsRepo()
			capiDiegoProcessAssociationsRepo = &models.CAPIDiegoProcessAssociationsRepo{
				Repo: make(map[models.CAPIProcessGUID]*models.CAPIDiegoProcessAssociation),
			}
			routesRepo.Upsert(&models.Route{
				GUID: "internal-route-guid-a",
				Host: "route-a.apps.internal",
			})
			routesRepo.Upsert(&models.Route{
				GUID: "internal-route-guid-b",
				Host: "route-b.apps.internal",
			})
			routeMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-a",
				CAPIProcessGUID: "capi-process-guid-a",
			})
			routeMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-b",
				CAPIProcessGUID: "capi-process-guid-b",
			})
			capiDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-a",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-a",
				},
			})
			capiDiegoProcessAssociationsRepo.Upsert(&models.CAPIDiegoProcessAssociation{
				CAPIProcessGUID: "capi-process-guid-b",
				DiegoProcessGUIDs: models.DiegoProcessGUIDs{
					"diego-process-guid-b",
				},
			})
			vipProvider = &fakes.VIPProvider{}
			vipProvider.GetStub = func(hostname string) string {
				return map[string]string{
					"route-a.apps.internal": "vip-for-route-a",
					"route-b.apps.internal": "vip-for-route-b",
				}[hostname]
			}

			internalRoutesRepo = &internalroutes.Repo{
				BackendSetRepo:                   backendSetRepo,
				Logger:                           logger,
				RoutesRepo:                       routesRepo,
				RouteMappingsRepo:                routeMappingsRepo,
				CAPIDiegoProcessAssociationsRepo: capiDiegoProcessAssociationsRepo,
				VIPProvider:                      vipProvider,
			}
		})

		It("returns the internal routes for each running backend instance", func() {
			backendSetRepo.GetInternalBackendsStub = func(guid models.DiegoProcessGUID) *models.BackendSet {
				diegoClientMap := map[models.DiegoProcessGUID]*models.BackendSet{
					"diego-process-guid-b": &models.BackendSet{
						Backends: []*models.Backend{
							{Address: "10.255.9.34", Port: 8080},
							{Address: "10.255.9.16", Port: 8080},
						},
					},
					"diego-process-guid-a": &models.BackendSet{
						Backends: []*models.Backend{
							{Address: "10.255.0.16", Port: 8080},
							{Address: "10.255.1.34", Port: 9080},
						},
					},
				}
				return diegoClientMap[guid]
			}

			routeAKey := internalroutes.InternalRoute{Hostname: "route-a.apps.internal", VIP: "vip-for-route-a"}
			routeBKey := internalroutes.InternalRoute{Hostname: "route-b.apps.internal", VIP: "vip-for-route-b"}

			internalRoutes, err := internalRoutesRepo.Get()
			Expect(err).NotTo(HaveOccurred())
			Expect(internalRoutes).To(HaveLen(2))

			Expect(internalRoutes).To(HaveKey(routeAKey))
			Expect(internalRoutes[routeAKey]).To(ConsistOf([]internalroutes.Backend{
				{
					Address: "10.255.0.16",
					Port:    8080,
				},
				{
					Address: "10.255.1.34",
					Port:    9080,
				},
			}))

			Expect(internalRoutes).To(HaveKey(routeBKey))
			Expect(internalRoutes[routeBKey]).To(ConsistOf([]internalroutes.Backend{
				{
					Address: "10.255.9.16",
					Port:    8080,
				},
				{
					Address: "10.255.9.34",
					Port:    8080,
				},
			}))

			routeMappingsRepo.Map(&models.RouteMapping{
				RouteGUID:       "internal-route-guid-a",
				CAPIProcessGUID: "capi-process-guid-b",
			})

			internalRoutes, err = internalRoutesRepo.Get()
			Expect(err).NotTo(HaveOccurred())
			Expect(internalRoutes).To(HaveLen(2))

			Expect(internalRoutes).To(HaveKey(routeAKey))
			Expect(internalRoutes[routeAKey]).To(ConsistOf([]internalroutes.Backend{
				{
					Address: "10.255.0.16",
					Port:    8080,
				},
				{
					Address: "10.255.1.34",
					Port:    9080,
				},
				{
					Address: "10.255.9.16",
					Port:    8080,
				},
				{
					Address: "10.255.9.34",
					Port:    8080,
				},
			}))

			Expect(internalRoutes).To(HaveKey(routeBKey))
			Expect(internalRoutes[routeBKey]).To(ConsistOf([]internalroutes.Backend{
				{
					Address: "10.255.9.16",
					Port:    8080,
				},
				{
					Address: "10.255.9.34",
					Port:    8080,
				},
			}))
		})

		Context("when GetInternalBackends returns nil", func() {
			It("skips the GUID and continues", func() {
				backendSetRepo.GetInternalBackendsReturns(nil)
				Expect(func() { internalRoutesRepo.Get() }).ShouldNot(Panic())
			})
		})
	})
})
