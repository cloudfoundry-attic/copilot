package integration_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"code.cloudfoundry.org/bbs/events"
	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"

	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Copilot", func() {
	var (
		session                        *gexec.Session
		istioClient                    copilot.IstioClient
		ccClient                       copilot.CloudControllerClient
		serverConfig                   *config.Config
		pilotClientTLSConfig           *tls.Config
		cloudControllerClientTLSConfig *tls.Config
		configFilePath                 string

		bbsServer    *ghttp.Server
		cleanupFuncs []func()
	)

	BeforeEach(func() {
		copilotCreds := testhelpers.GenerateMTLS()
		cleanupFuncs = append(cleanupFuncs, copilotCreds.CleanupTempFiles)

		listenAddrForPilot := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
		listenAddrForCloudController := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
		copilotTLSFiles := copilotCreds.CreateServerTLSFiles()

		bbsCreds := testhelpers.GenerateMTLS()
		cleanupFuncs = append(cleanupFuncs, copilotCreds.CleanupTempFiles)

		bbsTLSFiles := bbsCreds.CreateClientTLSFiles()

		// boot a fake BBS
		bbsServer = ghttp.NewUnstartedServer()
		bbsServer.HTTPTestServer.TLS = bbsCreds.ServerTLSConfig()

		bbsServer.RouteToHandler("GET", "/v1/events", func(w http.ResponseWriter, req *http.Request) {
			lrpEvent := bbsmodels.NewActualLRPCreatedEvent(&bbsmodels.ActualLRPGroup{
				Instance: &bbsmodels.ActualLRP{
					ActualLRPKey: bbsmodels.ActualLRPKey{
						ProcessGuid: "diego-process-guid-a",
					},
					State: bbsmodels.ActualLRPStateRunning,
					ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
						Address:         "10.10.1.3",
						InstanceAddress: "10.255.1.13",
						Ports: []*bbsmodels.PortMapping{
							{ContainerPort: 8080, HostPort: 61003},
						},
					},
				},
			})
			w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Add("Connection", "keep-alive")
			w.Header().Set("Transfer-Encoding", "identity")
			w.WriteHeader(http.StatusOK)

			conn, rw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}

			defer func() {
				conn.Close()
			}()

			rw.Flush()

			event, _ := events.NewEventFromModelEvent(0, lrpEvent)
			event.Write(conn)
		})
		bbsServer.RouteToHandler("POST", "/v1/cells/list.r1", func(w http.ResponseWriter, req *http.Request) {
			cellsResponse := bbsmodels.CellsResponse{}
			data, _ := proto.Marshal(&cellsResponse)
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		})
		bbsServer.RouteToHandler("POST", "/v1/actual_lrp_groups/list", func(w http.ResponseWriter, req *http.Request) {
			actualLRPResponse := bbsmodels.ActualLRPGroupsResponse{
				ActualLrpGroups: []*bbsmodels.ActualLRPGroup{
					{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.ActualLRPKey{
								ProcessGuid: "diego-process-guid-a",
							},
							State: bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address:         "10.10.1.3",
								InstanceAddress: "10.255.1.13",
								Ports: []*bbsmodels.PortMapping{
									{ContainerPort: 8080, HostPort: 61003},
								},
							},
						},
					},
					{ // this instance only has SSH port, not app port.  it shouldn't show up in route results
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-a", 1, "domain1"),
							State:        bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address:         "10.10.1.4",
								InstanceAddress: "10.255.1.15",
								Ports: []*bbsmodels.PortMapping{
									{ContainerPort: 2222, HostPort: 61004},
								},
							},
						},
					},
					{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-a", 1, "domain1"),
							State:        bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address:         "10.10.1.5",
								InstanceAddress: "10.255.1.16",
								Ports: []*bbsmodels.PortMapping{
									{ContainerPort: 8080, HostPort: 61005},
								},
							},
						},
					},
					{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-b", 1, "domain1"),
							State:        bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address:         "10.10.1.6",
								InstanceAddress: "10.255.0.34",
								Ports: []*bbsmodels.PortMapping{
									{ContainerPort: 2222, HostPort: 61008},
									{ContainerPort: 8080, HostPort: 61006},
								},
							},
						},
					},
					{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.NewActualLRPKey("diego-process-guid-other", 1, "domain1"),
							State:        bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address:         "10.10.1.7",
								InstanceAddress: "10.255.0.35",
								Ports: []*bbsmodels.PortMapping{
									{ContainerPort: 8080, HostPort: 61111},
								},
							},
						},
					},
				},
			}
			data, _ := proto.Marshal(&actualLRPResponse)
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		})
		bbsServer.Start()
		cleanupFuncs = append(cleanupFuncs, bbsServer.Close)

		serverConfig = &config.Config{
			ListenAddressForPilot:           listenAddrForPilot,
			ListenAddressForCloudController: listenAddrForCloudController,
			PilotClientCAPath:               copilotTLSFiles.ClientCA,
			CloudControllerClientCAPath:     copilotTLSFiles.OtherClientCA,
			ServerCertPath:                  copilotTLSFiles.ServerCert,
			ServerKeyPath:                   copilotTLSFiles.ServerKey,
			VIPCIDR:                         "127.128.0.0/9",
			BBS: &config.BBSConfig{
				ServerCACertPath: bbsTLSFiles.ServerCA,
				ClientCertPath:   bbsTLSFiles.ClientCert,
				ClientKeyPath:    bbsTLSFiles.ClientKey,
				Address:          bbsServer.URL(),
				SyncInterval:     "10ms",
			},
		}

		configFilePath = testhelpers.TempFileName()
		cleanupFuncs = append(cleanupFuncs, func() { os.Remove(configFilePath) })

		Expect(serverConfig.Save(configFilePath)).To(Succeed())

		cmd := exec.Command(binaryPath, "-config", configFilePath)
		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session.Out).Should(gbytes.Say(`started`))

		pilotClientTLSConfig = copilotCreds.ClientTLSConfig()
		cloudControllerClientTLSConfig = copilotCreds.OtherClientTLSConfig()

		istioClient, err = copilot.NewIstioClient(serverConfig.ListenAddressForPilot, pilotClientTLSConfig)
		Expect(err).NotTo(HaveOccurred())
		ccClient, err = copilot.NewCloudControllerClient(serverConfig.ListenAddressForCloudController, cloudControllerClientTLSConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		session.Interrupt()
		Eventually(session, "10s").Should(gexec.Exit())

		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	})

	Specify("a journey", func() {
		WaitForHealthy(istioClient, ccClient)

		By("CC creates and maps a route")
		_, err := ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
			Route: &api.Route{
				Guid: "route-guid-a",
				Host: "some-url",
			}})

		Expect(err).NotTo(HaveOccurred())
		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "route-guid-a",
				CapiProcessGuid: "capi-process-guid-a",
			},
		})

		Expect(err).NotTo(HaveOccurred())
		_, err = ccClient.UpsertCapiDiegoProcessAssociation(context.Background(), &api.UpsertCapiDiegoProcessAssociationRequest{
			CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
				CapiProcessGuid: "capi-process-guid-a",
				DiegoProcessGuids: []string{
					"diego-process-guid-a",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("istio client sees that route")
		istioVisibleRoutes, err := istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())

		Expect(istioVisibleRoutes.Routes).To(HaveLen(1))
		route := istioVisibleRoutes.Routes[0]
		Expect(route.Hostname).To(Equal("some-url"))
		Expect(route.Backends).To(Equal(&api.BackendSet{
			Backends: []*api.Backend{
				{Address: "10.10.1.3", Port: 61003},
				{Address: "10.10.1.5", Port: 61005},
			},
		}))
		Expect(route.CapiProcessGuid).To(Equal("capi-process-guid-a"))
		Expect(route.Path).To(BeEmpty())

		By("cc maps another backend to the same route")
		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "route-guid-a",
				CapiProcessGuid: "capi-process-guid-b",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.UpsertCapiDiegoProcessAssociation(context.Background(), &api.UpsertCapiDiegoProcessAssociationRequest{
			CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
				CapiProcessGuid: "capi-process-guid-b",
				DiegoProcessGuids: []string{
					"diego-process-guid-b",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("cc adds a second route and maps it to the second backend")
		_, err = ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
			Route: &api.Route{
				Guid: "route-guid-b",
				Host: "some-url",
				Path: "/some/path",
			}})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "route-guid-b",
				CapiProcessGuid: "capi-process-guid-other",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.UpsertCapiDiegoProcessAssociation(context.Background(), &api.UpsertCapiDiegoProcessAssociationRequest{
			CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
				CapiProcessGuid: "capi-process-guid-other",
				DiegoProcessGuids: []string{
					"diego-process-guid-other",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("istio client sees both routes and their respective backends")
		istioVisibleRoutes, err = istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())

		routes := istioVisibleRoutes.Routes
		Expect(routes).To(HaveLen(3))
		Expect(istioVisibleRoutes.Routes).To(ConsistOf([]*api.RouteWithBackends{
			&api.RouteWithBackends{
				Hostname: "some-url",
				Path:     "/some/path",
				Backends: &api.BackendSet{
					Backends: []*api.Backend{
						&api.Backend{Address: "10.10.1.7", Port: 61111},
					},
				},
				CapiProcessGuid: "capi-process-guid-other",
			},
			&api.RouteWithBackends{
				Hostname: "some-url",
				Backends: &api.BackendSet{
					Backends: []*api.Backend{
						&api.Backend{Address: "10.10.1.3", Port: 61003},
						&api.Backend{Address: "10.10.1.5", Port: 61005},
					},
				},
				CapiProcessGuid: "capi-process-guid-a",
			},
			&api.RouteWithBackends{
				Hostname: "some-url",
				Backends: &api.BackendSet{
					Backends: []*api.Backend{
						&api.Backend{Address: "10.10.1.6", Port: 61006},
					},
				},
				CapiProcessGuid: "capi-process-guid-b",
			},
		}))

		By("cc unmaps the first backend from the first route")
		_, err = ccClient.UnmapRoute(context.Background(), &api.UnmapRouteRequest{RouteMapping: &api.RouteMapping{
			RouteGuid:       "route-guid-a",
			CapiProcessGuid: "capi-process-guid-a",
		}})
		Expect(err).NotTo(HaveOccurred())

		By("cc delete the second route")
		_, err = ccClient.DeleteRoute(context.Background(), &api.DeleteRouteRequest{
			Guid: "route-guid-b",
		})
		Expect(err).NotTo(HaveOccurred())

		istioVisibleRoutes, err = istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		By("istio client sees the updated stuff")
		Expect(istioVisibleRoutes.Routes).To(HaveLen(1))
		route = istioVisibleRoutes.Routes[0]
		Expect(route.Hostname).To(Equal("some-url"))
		Expect(route.Backends.Backends).To(ConsistOf(
			&api.Backend{Address: "10.10.1.6", Port: 61006},
		))

		By("cc maps an internal route")
		_, err = ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
			Route: &api.Route{
				Guid: "internal-route-guid",
				Host: "route.apps.internal",
			}})
		Expect(err).NotTo(HaveOccurred())
		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "internal-route-guid",
				CapiProcessGuid: "capi-process-guid-b",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("istio client sees internal routes")
		istioVisibleRoutes, err = istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		istioVisibleInternalRoutes, err := istioClient.InternalRoutes(context.Background(), new(api.InternalRoutesRequest))
		Expect(err).NotTo(HaveOccurred())

		Expect(istioVisibleRoutes.Routes).To(HaveLen(1))
		//The list of backends does not have a guaranteed order, this test is flakey if you assert on the whole set of Routes at once
		route = istioVisibleRoutes.Routes[0]
		Expect(route.Hostname).To(Equal("some-url"))
		Expect(route.Backends.Backends).To(ConsistOf(
			&api.Backend{Address: "10.10.1.6", Port: 61006},
		))

		Expect(istioVisibleInternalRoutes.InternalRoutes).To(HaveLen(1))
		internalRoute := istioVisibleInternalRoutes.InternalRoutes[0]
		Expect(internalRoute.Hostname).To(Equal("route.apps.internal"))
		Expect(internalRoute.Vip).To(Equal("127.138.254.35")) // magic number, if you change the VIP provider, you'll need to change this too!

		By("checking that the backend for the capi process is returned for that route")
		Expect(internalRoute.Backends.Backends).To(ConsistOf(
			&api.Backend{Address: "10.255.0.34", Port: 8080},
		))

		By("mapping another capi process to the same internal route")
		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "internal-route-guid",
				CapiProcessGuid: "capi-process-guid-a",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		istioVisibleInternalRoutes, err = istioClient.InternalRoutes(context.Background(), new(api.InternalRoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		internalRoute = istioVisibleInternalRoutes.InternalRoutes[0]
		Expect(istioVisibleInternalRoutes.InternalRoutes).To(HaveLen(1))
		Expect(internalRoute.Hostname).To(Equal("route.apps.internal"))
		Expect(internalRoute.Vip).To(Equal("127.138.254.35")) // magic number, if you change the VIP provider, you'll need to change this too!
		By("checking that backends for both capi processes are returned for that route")
		Expect(internalRoute.Backends.Backends).To(ConsistOf(
			&api.Backend{Address: "10.255.1.16", Port: 8080},
			&api.Backend{Address: "10.255.1.13", Port: 8080},
			&api.Backend{Address: "10.255.0.34", Port: 8080},
		))
	})

	Context("when the vip cidr is invalid", func() {
		BeforeEach(func() {
			// stop copilot
			session.Interrupt()
			Eventually(session, "2s").Should(gexec.Exit())
			serverConfig.VIPCIDR = "not an ip"
			Expect(serverConfig.Save(configFilePath)).To(Succeed())
		})

		It("exits with a helpful error message", func() {
			cmd := exec.Command(binaryPath, "-config", configFilePath)
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "2s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say(`parsing vip cidr: invalid CIDR address: not an ip`))
		})
	})

	Context("when the BBS is not available", func() {
		BeforeEach(func() {
			bbsServer.Close()

			// stop copilot
			session.Interrupt()
			Eventually(session, "2s").Should(gexec.Exit())
		})

		It("crashes and prints a useful error log", func() {
			// re-start copilot
			cmd := exec.Command(binaryPath, "-config", configFilePath)
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, "2s").Should(gexec.Exit(1))
			Eventually(session.Out).Should(gbytes.Say(`unable to reach BBS`))
		})

		Context("but if the user sets config BBS.Disable", func() {
			BeforeEach(func() {
				serverConfig.BBS.Disable = true
				Expect(serverConfig.Save(configFilePath)).To(Succeed())
			})

			It("boots successfully and serves requests on the Cloud Controller-facing server", func() {
				cmd := exec.Command(binaryPath, "-config", configFilePath)
				var err error
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say(`BBS is disabled`))

				WaitForHealthy(istioClient, ccClient)
				_, err = ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
					Route: &api.Route{
						Guid: "route-guid-xyz",
						Host: "some-url",
					}})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	It("gracefully terminates when sent an interrupt signal", func() {
		WaitForHealthy(istioClient, ccClient)
		Consistently(session, "1s").ShouldNot(gexec.Exit())
		_, err := istioClient.Health(context.Background(), new(api.HealthRequest))
		Expect(err).NotTo(HaveOccurred())

		Expect(istioClient.Close()).To(Succeed())
		session.Interrupt()

		Eventually(session, "2s").Should(gexec.Exit())
	})

	Context("when the pilot-facing server tls config is invalid", func() {
		BeforeEach(func() {
			pilotClientTLSConfig.RootCAs = nil
			var err error
			istioClient, err = copilot.NewIstioClient(serverConfig.ListenAddressForPilot, pilotClientTLSConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		Specify("the istioClient gets a meaningful error", func() {
			_, err := istioClient.Health(context.Background(), new(api.HealthRequest))
			Expect(err).To(MatchError(ContainSubstring("authentication handshake failed")))
		})
	})
})

func WaitForHealthy(istioClient copilot.IstioClient, ccClient copilot.CloudControllerClient) {
	By("waiting for the servers to become healthy")
	serverForPilotIsHealthy := func() error {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelFunc()
		_, err := istioClient.Health(ctx, new(api.HealthRequest))
		return err
	}
	Eventually(serverForPilotIsHealthy, 2*time.Second).Should(Succeed())

	serverForCloudControllerIsHealthy := func() error {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelFunc()
		_, err := ccClient.Health(ctx, new(api.HealthRequest))
		return err
	}
	Eventually(serverForCloudControllerIsHealthy, 2*time.Second).Should(Succeed())
}
