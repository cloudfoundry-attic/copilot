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
							ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-a", 1, "domain1"),
							State:        bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address: "10.10.1.5",
								Ports: []*bbsmodels.PortMapping{
									{ContainerPort: 8080, HostPort: 61005},
								},
							},
						},
					},
					{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-b", 1, "domain1"),
							State:        bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address: "10.10.1.6",
								Ports: []*bbsmodels.PortMapping{
									{ContainerPort: 8080, HostPort: 61006},
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
			BBS: config.BBSConfig{
				ServerCACertPath: bbsTLSFiles.ServerCA,
				ClientCertPath:   bbsTLSFiles.ClientCert,
				ClientKeyPath:    bbsTLSFiles.ClientKey,
				Address:          bbsServer.URL(),
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
		Eventually(session, "2s").Should(gexec.Exit())

		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	})

	It("serves internal routes using data from the BBS, even when there are no CAPI-provided routes", func() {
		WaitForHealthy(istioClient, ccClient)
		routes, err := istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		Expect(routes.Backends).To(Equal(map[string]*api.BackendSet{
			"process-guid-a.cfapps.internal": {
				Backends: []*api.Backend{
					{Address: "10.10.1.5", Port: 61005},
				},
			},
			"process-guid-b.cfapps.internal": {
				Backends: []*api.Backend{
					{Address: "10.10.1.6", Port: 61006},
				},
			},
		}))
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
				RouteGuid: "route-guid-a",
				CapiProcess: &api.CapiProcess{
					DiegoProcessGuid: "process-guid-a",
					Guid:             "capi-process-guid-a",
				}},
		})
		Expect(err).NotTo(HaveOccurred())

		By("istio client sees that route")
		istioVisibleRoutes, err := istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		Expect(istioVisibleRoutes.Backends).To(Equal(map[string]*api.BackendSet{
			"process-guid-a.cfapps.internal": {
				Backends: []*api.Backend{
					{Address: "10.10.1.5", Port: 61005},
				},
			},
			"process-guid-b.cfapps.internal": {
				Backends: []*api.Backend{
					{Address: "10.10.1.6", Port: 61006},
				},
			},
			"some-url": {
				Backends: []*api.Backend{
					{Address: "10.10.1.5", Port: 61005},
				},
			},
		}))

		By("cc maps another backend to the same route")
		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid: "route-guid-a",
				CapiProcess: &api.CapiProcess{
					DiegoProcessGuid: "process-guid-b",
					Guid:             "capi-process-guid-b",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("cc adds a second route and maps it to the second backend")
		_, err = ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
			Route: &api.Route{
				Guid: "route-guid-b",
				Host: "some-url-b",
			}})
		Expect(err).NotTo(HaveOccurred())
		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid: "route-guid-b",
				CapiProcess: &api.CapiProcess{
					DiegoProcessGuid: "process-guid-b",
					Guid:             "capi-process-guid-b",
				}},
		})
		Expect(err).NotTo(HaveOccurred())

		By("istio client sees that new stuff")
		istioVisibleRoutes, err = istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		Expect(istioVisibleRoutes.Backends).To(HaveLen(4))
		Expect(istioVisibleRoutes.Backends["process-guid-a.cfapps.internal"].Backends).To(ConsistOf(
			&api.Backend{Address: "10.10.1.5", Port: 61005},
		))
		Expect(istioVisibleRoutes.Backends["process-guid-b.cfapps.internal"].Backends).To(ConsistOf(
			&api.Backend{Address: "10.10.1.6", Port: 61006},
		))
		//The list of backends does not have a guaranteed order, this test is flakey if you assert on the whole set of Routes at once
		Expect(istioVisibleRoutes.Backends["some-url"].Backends).To(ConsistOf(
			&api.Backend{Address: "10.10.1.5", Port: 61005},
			&api.Backend{Address: "10.10.1.6", Port: 61006},
		))
		Expect(istioVisibleRoutes.Backends["some-url-b"].Backends).To(ConsistOf(
			&api.Backend{Address: "10.10.1.6", Port: 61006},
		))

		By("cc unmaps the first backend from the first route")
		_, err = ccClient.UnmapRoute(context.Background(), &api.UnmapRouteRequest{
			RouteGuid:       "route-guid-a",
			CapiProcessGuid: "capi-process-guid-a",
		})
		Expect(err).NotTo(HaveOccurred())

		By("cc delete the second route")
		_, err = ccClient.DeleteRoute(context.Background(), &api.DeleteRouteRequest{
			Guid: "route-guid-b",
		})
		Expect(err).NotTo(HaveOccurred())

		istioVisibleRoutes, err = istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		By("istio client sees the updated stuff")
		Expect(istioVisibleRoutes.Backends).To(Equal(map[string]*api.BackendSet{
			"process-guid-a.cfapps.internal": {
				Backends: []*api.Backend{
					{Address: "10.10.1.5", Port: 61005},
				},
			},
			"process-guid-b.cfapps.internal": {
				Backends: []*api.Backend{
					{Address: "10.10.1.6", Port: 61006},
				},
			},
			"some-url": {
				Backends: []*api.Backend{
					{Address: "10.10.1.6", Port: 61006},
				},
			},
		}))
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
			Expect(session.Out).To(gbytes.Say(`unable to reach BBS`))
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
