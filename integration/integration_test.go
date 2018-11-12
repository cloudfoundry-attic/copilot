package integration_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/exec"
	"time"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
	copilotsnapshot "code.cloudfoundry.org/copilot/snapshot"
	"code.cloudfoundry.org/copilot/testhelpers"
	"code.cloudfoundry.org/durationjson"
	"github.com/gogo/protobuf/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"istio.io/api/networking/v1alpha3"
)

var _ = Describe("Copilot", func() {
	var (
		session                        *gexec.Session
		serverConfig                   *config.Config
		pilotClientTLSConfig           *tls.Config
		cloudControllerClientTLSConfig *tls.Config
		configFilePath                 string

		mcpClient *testhelpers.MockPilotMCPClient
		ccClient  copilot.CloudControllerClient
		mockBBS   *testhelpers.MockBBSServer

		cleanupFuncs      []func()
		routeHost         string
		internalRouteHost string
	)

	BeforeEach(func() {
		mockBBS = testhelpers.NewMockBBSServer()
		mockBBS.SetGetV1EventsResponse(&bbsmodels.ActualLRPGroup{
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

		mockBBS.SetPostV1ActualLRPGroupsList(
			[]*bbsmodels.ActualLRPGroup{
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
			})
		mockBBS.Server.Start()
		cleanupFuncs = append(cleanupFuncs, mockBBS.Server.Close)

		copilotCreds := testhelpers.GenerateMTLS()
		cleanupFuncs = append(cleanupFuncs, copilotCreds.CleanupTempFiles)
		listenAddrForCloudController := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
		listenAddrForMCP := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
		copilotTLSFiles := copilotCreds.CreateServerTLSFiles()
		bbsCreds := testhelpers.GenerateMTLS()
		bbsTLSFiles := bbsCreds.CreateClientTLSFiles()
		mockBBS.Server.HTTPTestServer.TLS = bbsCreds.ServerTLSConfig()

		serverConfig = &config.Config{
			ListenAddressForCloudController: listenAddrForCloudController,
			ListenAddressForMCP:             listenAddrForMCP,
			PilotClientCAPath:               copilotTLSFiles.ClientCA,
			CloudControllerClientCAPath:     copilotTLSFiles.OtherClientCA,
			ServerCertPath:                  copilotTLSFiles.ServerCert,
			ServerKeyPath:                   copilotTLSFiles.ServerKey,
			VIPCIDR:                         "127.128.0.0/9",
			MCPConvergeInterval:             durationjson.Duration(10 * time.Millisecond),
			BBS: &config.BBSConfig{
				ServerCACertPath: bbsTLSFiles.ServerCA,
				ClientCertPath:   bbsTLSFiles.ClientCert,
				ClientKeyPath:    bbsTLSFiles.ClientKey,
				Address:          mockBBS.Server.URL(),
				SyncInterval:     durationjson.Duration(10 * time.Millisecond),
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

		ccClient, err = copilot.NewCloudControllerClient(serverConfig.ListenAddressForCloudController, cloudControllerClientTLSConfig)
		Expect(err).NotTo(HaveOccurred())
		mcpClient, err = testhelpers.NewMockPilotMCPClient(pilotClientTLSConfig, serverConfig.ListenAddressForMCP)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := mcpClient.Close()
		Expect(err).To(BeNil())
		session.Interrupt()
		Eventually(session, "10s").Should(gexec.Exit())

		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	})

	Specify("a journey", func() {
		WaitForHealthy(ccClient)

		By("CC creates and maps a route")
		routeHost = "some-url"
		_, err := ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
			Route: &api.Route{
				Guid: "route-guid-a",
				Host: routeHost,
			}})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "route-guid-a",
				CapiProcessGuid: "capi-process-guid-a",
				RouteWeight:     1,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.UpsertCapiDiegoProcessAssociation(context.Background(), &api.UpsertCapiDiegoProcessAssociationRequest{
			CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
				CapiProcessGuid:   "capi-process-guid-a",
				DiegoProcessGuids: []string{"diego-process-guid-a"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("CC creates and maps an internal route")
		internalRouteHost = "some-internal-url"
		_, err = ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
			Route: &api.Route{
				Guid:     "internal-route-guid-a",
				Host:     internalRouteHost,
				Internal: true,
			}})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "internal-route-guid-a",
				CapiProcessGuid: "capi-process-guid-a",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.UpsertCapiDiegoProcessAssociation(context.Background(), &api.UpsertCapiDiegoProcessAssociationRequest{
			CapiDiegoProcessAssociation: &api.CapiDiegoProcessAssociation{
				CapiProcessGuid:   "capi-process-guid-a",
				DiegoProcessGuids: []string{"diego-process-guid-a"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("istio pilot MCP client sees the correct messages and objects")
		Eventually(mcpClient.GetAllMessageNames, "1s").Should(ConsistOf(
			"type.googleapis.com/istio.networking.v1alpha3.DestinationRule",
			"type.googleapis.com/istio.networking.v1alpha3.VirtualService",
			"type.googleapis.com/istio.networking.v1alpha3.ServiceEntry",
			"type.googleapis.com/istio.networking.v1alpha3.Gateway",
		))

		Eventually(mcpClient.GetAllObjectNames, "1s").Should(Equal(map[string][]string{
			"type.googleapis.com/istio.networking.v1alpha3.DestinationRule": []string{fmt.Sprintf("copilot-rule-for-%s", routeHost)},
			"type.googleapis.com/istio.networking.v1alpha3.VirtualService":  []string{fmt.Sprintf("copilot-service-for-%s", routeHost), fmt.Sprintf("copilot-service-for-%s", internalRouteHost)},
			"type.googleapis.com/istio.networking.v1alpha3.ServiceEntry":    []string{fmt.Sprintf("copilot-service-entry-for-%s", routeHost), fmt.Sprintf("copilot-service-entry-for-%s", internalRouteHost)},
			"type.googleapis.com/istio.networking.v1alpha3.Gateway":         []string{copilotsnapshot.DefaultGatewayName},
		}))

		expectedRoutes := []Route{
			{
				dest: generateDestination([]RouteDestination{
					{
						port:   8080,
						weight: 100,
						subset: "capi-process-guid-a",
						host:   "some-url",
					},
				}),
			},
		}
		expectedRoutesInternal := []Route{
			{
				dest: generateDestination([]RouteDestination{
					{
						port:   8080,
						weight: 100,
						subset: "capi-process-guid-a",
						host:   "some-internal-url",
					},
				}),
			},
		}
		expectedVS := expectedVirtualService("some-url", "cloudfoundry-ingress", expectedRoutes)
		expectedInternalVS := expectedVirtualServiceWithRetries("some-internal-url", "", expectedRoutesInternal)
		Eventually(mcpClient.GetAllVirtualServices, "1s").Should(ConsistOf(expectedVS, expectedInternalVS))

		expectedDR := expectedDestinationRule("some-url", []string{"capi-process-guid-a"})
		Eventually(mcpClient.GetAllDestinationRules, "1s").Should(Equal([]*v1alpha3.DestinationRule{expectedDR}))

		expectedGW := expectedGateway(80)
		Eventually(mcpClient.GetAllGateways, "1s").Should(Equal([]*v1alpha3.Gateway{expectedGW}))

		expectedSE := expectedServiceEntry(
			"some-url",
			"",
			"http",
			[]Endpoint{
				{
					port:   61003,
					addr:   "10.10.1.3",
					subset: "capi-process-guid-a",
				},
				{
					port:   61005,
					addr:   "10.10.1.5",
					subset: "capi-process-guid-a",
				},
			},
		)
		expectedInternalSE := expectedServiceEntry(
			"some-internal-url",
			"127.175.61.18",
			"tcp",
			[]Endpoint{
				{
					port:   61003,
					addr:   "10.10.1.3",
					subset: "capi-process-guid-a",
				},
				{
					port:   61005,
					addr:   "10.10.1.5",
					subset: "capi-process-guid-a",
				},
			},
		)
		Eventually(mcpClient.GetAllServiceEntries, "1s").Should(ConsistOf(expectedSE, expectedInternalSE))

		By("cc maps another backend to the same route")
		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "route-guid-a",
				CapiProcessGuid: "capi-process-guid-b",
				RouteWeight:     1,
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
				Host: routeHost,
				Path: "/some/path",
			}})
		Expect(err).NotTo(HaveOccurred())

		_, err = ccClient.MapRoute(context.Background(), &api.MapRouteRequest{
			RouteMapping: &api.RouteMapping{
				RouteGuid:       "route-guid-b",
				CapiProcessGuid: "capi-process-guid-other",
				RouteWeight:     1,
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

		By("istio mcp client sees both routes and their respective backends")
		expectedSE = expectedServiceEntry(
			"some-url",
			"",
			"http",
			[]Endpoint{
				{
					port:   61111,
					addr:   "10.10.1.7",
					subset: "capi-process-guid-other",
				},
				{
					port:   61003,
					addr:   "10.10.1.3",
					subset: "capi-process-guid-a",
				},
				{
					port:   61005,
					addr:   "10.10.1.5",
					subset: "capi-process-guid-a",
				},
				{
					port:   61006,
					addr:   "10.10.1.6",
					subset: "capi-process-guid-b",
				},
			},
		)
		Eventually(mcpClient.GetAllServiceEntries, "1s").Should(ConsistOf(expectedSE, expectedInternalSE))

		expectedRoutes = []Route{
			{
				dest: generateDestination([]RouteDestination{
					{
						port:   8080,
						weight: 100,
						subset: "capi-process-guid-other",
						host:   "some-url",
					},
				}),
				match: generateMatch([]string{"/some/path"}),
			},
			{
				dest: generateDestination([]RouteDestination{
					{
						port:   8080,
						weight: 50,
						subset: "capi-process-guid-a",
						host:   "some-url",
					},
					{
						port:   8080,
						weight: 50,
						subset: "capi-process-guid-b",
						host:   "some-url",
					},
				}),
			},
		}
		expectedVS = expectedVirtualService("some-url", "cloudfoundry-ingress", expectedRoutes)
		Eventually(mcpClient.GetAllVirtualServices, "1s").Should(ConsistOf(
			expectedVS,
			expectedInternalVS,
		))

		expectedDR = expectedDestinationRule("some-url",
			[]string{"capi-process-guid-other", "capi-process-guid-a", "capi-process-guid-b"})
		Eventually(mcpClient.GetAllDestinationRules, "1s").Should(Equal([]*v1alpha3.DestinationRule{expectedDR}))

		expectedGW = expectedGateway(80)
		Eventually(mcpClient.GetAllGateways, "1s").Should(Equal([]*v1alpha3.Gateway{expectedGW}))

		By("cc unmaps the first backend from the first route")
		_, err = ccClient.UnmapRoute(context.Background(), &api.UnmapRouteRequest{RouteMapping: &api.RouteMapping{
			RouteGuid:       "route-guid-a",
			CapiProcessGuid: "capi-process-guid-a",
			RouteWeight:     1,
		}})
		Expect(err).NotTo(HaveOccurred())

		By("cc deletes the second route")
		_, err = ccClient.DeleteRoute(context.Background(), &api.DeleteRouteRequest{
			Guid: "route-guid-b",
		})
		Expect(err).NotTo(HaveOccurred())

		By("istio mcp client sees the updated stuff")
		expectedSE = expectedServiceEntry(
			"some-url",
			"",
			"http",
			[]Endpoint{
				{
					port:   61006,
					addr:   "10.10.1.6",
					subset: "capi-process-guid-b",
				},
			},
		)
		Eventually(mcpClient.GetAllServiceEntries, "1s").Should(ConsistOf(expectedSE, expectedInternalSE))

		expectedRoutes = []Route{
			{
				dest: generateDestination([]RouteDestination{
					{
						port:   8080,
						weight: 100,
						subset: "capi-process-guid-b",
						host:   "some-url",
					},
				}),
			},
		}
		expectedVS = expectedVirtualService("some-url", "cloudfoundry-ingress", expectedRoutes)
		Eventually(mcpClient.GetAllVirtualServices, "3s").Should(ConsistOf(
			expectedVS,
			expectedInternalVS,
		))

		expectedDR = expectedDestinationRule("some-url",
			[]string{"capi-process-guid-b"})
		Eventually(mcpClient.GetAllDestinationRules, "1s").Should(Equal([]*v1alpha3.DestinationRule{expectedDR}))

		expectedGW = expectedGateway(80)
		Eventually(mcpClient.GetAllGateways, "1s").Should(Equal([]*v1alpha3.Gateway{expectedGW}))
	})

	Context("when the BBS is not available", func() {
		BeforeEach(func() {
			mockBBS.Server.Close()

			// stop copilot
			gexec.KillAndWait(time.Second * 10)
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
				Eventually(session.Out).Should(gbytes.Say(`BBS support is disabled`))

				WaitForHealthy(ccClient)
				_, err = ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
					Route: &api.Route{
						Guid: "route-guid-xyz",
						Host: "some-url",
					}})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

func WaitForHealthy(ccClient copilot.CloudControllerClient) {
	By("waiting for the server to become healthy")
	serverForCloudControllerIsHealthy := func() error {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelFunc()
		_, err := ccClient.Health(ctx, new(api.HealthRequest))
		return err
	}
	Eventually(serverForCloudControllerIsHealthy, 2*time.Second).Should(Succeed())
}

type Route struct {
	match []*v1alpha3.HTTPMatchRequest
	dest  []*v1alpha3.HTTPRouteDestination
}

type RouteDestination struct {
	host   string
	port   uint32
	weight int32
	subset string
}

func generateMatch(paths []string) []*v1alpha3.HTTPMatchRequest {
	matches := []*v1alpha3.HTTPMatchRequest{}

	for _, p := range paths {
		matches = append(matches, &v1alpha3.HTTPMatchRequest{
			Uri: &v1alpha3.StringMatch{
				MatchType: &v1alpha3.StringMatch_Prefix{Prefix: p},
			},
			Scheme:       nil,
			Method:       nil,
			Authority:    nil,
			Headers:      nil,
			Port:         0,
			SourceLabels: nil,
			Gateways:     nil,
		})
	}

	return matches
}

func generateDestination(dests []RouteDestination) []*v1alpha3.HTTPRouteDestination {
	newDests := []*v1alpha3.HTTPRouteDestination{}

	for _, d := range dests {
		newDests = append(newDests, &v1alpha3.HTTPRouteDestination{
			Destination: &v1alpha3.Destination{
				Host:   d.host,
				Subset: d.subset,
				Port: &v1alpha3.PortSelector{
					Port: &v1alpha3.PortSelector_Number{Number: d.port},
				},
			},
			Weight:                d.weight,
			RemoveResponseHeaders: nil,
			AppendResponseHeaders: nil,
			RemoveRequestHeaders:  nil,
			AppendRequestHeaders:  nil,
		})
	}

	return newDests
}

func expectedVirtualService(host, gateway string, routes []Route) *v1alpha3.VirtualService {
	newRoutes := []*v1alpha3.HTTPRoute{}
	for _, r := range routes {
		newRoutes = append(newRoutes, &v1alpha3.HTTPRoute{
			Match:                 r.match,
			Route:                 r.dest,
			Redirect:              nil,
			Rewrite:               nil,
			WebsocketUpgrade:      false,
			Timeout:               nil,
			Retries:               nil,
			Fault:                 nil,
			Mirror:                nil,
			CorsPolicy:            nil,
			AppendHeaders:         nil,
			RemoveResponseHeaders: nil,
			AppendResponseHeaders: nil,
			RemoveRequestHeaders:  nil,
			AppendRequestHeaders:  nil,
		})
	}

	var gateways []string
	if gateway != "" {
		gateways = []string{gateway}
	}
	return &v1alpha3.VirtualService{
		Hosts:    []string{host},
		Gateways: gateways,
		Tls:      nil,
		Tcp:      nil,
		Http:     newRoutes,
	}
}

func expectedVirtualServiceWithRetries(host, gateway string, routes []Route) *v1alpha3.VirtualService {
	newRoutes := []*v1alpha3.HTTPRoute{}
	for _, r := range routes {
		newRoutes = append(newRoutes, &v1alpha3.HTTPRoute{
			Match:            r.match,
			Route:            r.dest,
			Redirect:         nil,
			Rewrite:          nil,
			WebsocketUpgrade: false,
			Timeout:          nil,
			Retries: &v1alpha3.HTTPRetry{
				Attempts: 3,
				PerTryTimeout: &types.Duration{
					Nanos: 200,
				},
			},
			Fault:                 nil,
			Mirror:                nil,
			CorsPolicy:            nil,
			AppendHeaders:         nil,
			RemoveResponseHeaders: nil,
			AppendResponseHeaders: nil,
			RemoveRequestHeaders:  nil,
			AppendRequestHeaders:  nil,
		})
	}

	var gateways []string
	if gateway != "" {
		gateways = []string{gateway}
	}
	return &v1alpha3.VirtualService{
		Hosts:    []string{host},
		Gateways: gateways,
		Tls:      nil,
		Tcp:      nil,
		Http:     newRoutes,
	}
}

func expectedDestinationRule(host string, subsets []string) *v1alpha3.DestinationRule {
	sets := []*v1alpha3.Subset{}
	for _, s := range subsets {
		sets = append(sets, &v1alpha3.Subset{
			Name: s,
			Labels: map[string]string{
				"cfapp": s,
			},
			TrafficPolicy: nil,
		})
	}

	return &v1alpha3.DestinationRule{
		Host:          host,
		TrafficPolicy: nil,
		Subsets:       sets,
	}
}

func expectedGateway(port uint32) *v1alpha3.Gateway {
	return &v1alpha3.Gateway{
		Servers: []*v1alpha3.Server{
			{
				Port:  &v1alpha3.Port{Number: port, Protocol: "http", Name: "http"},
				Hosts: []string{"*"},
				Tls:   nil,
			},
		},
		Selector: nil,
	}
}

type Endpoint struct {
	addr   string
	port   uint32
	subset string
}

func expectedServiceEntry(host, address, protocol string, newEndpoints []Endpoint) *v1alpha3.ServiceEntry {
	endpoints := []*v1alpha3.ServiceEntry_Endpoint{}
	for i, _ := range newEndpoints {
		endpoints = append(endpoints, &v1alpha3.ServiceEntry_Endpoint{
			Address: newEndpoints[i].addr,
			Ports:   map[string]uint32{protocol: newEndpoints[i].port},
			Labels: map[string]string{
				"cfapp": newEndpoints[i].subset,
			},
		})
	}

	var addresses []string
	if address != "" {
		addresses = []string{address}
	}

	return &v1alpha3.ServiceEntry{
		Hosts:     []string{host},
		Addresses: addresses,
		Ports: []*v1alpha3.Port{
			{Number: 8080, Protocol: protocol, Name: protocol},
		},
		Location:   1,
		Resolution: 1,
		Endpoints:  endpoints,
	}
}
