package integration_test

import (
	"context"
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

	"google.golang.org/grpc"

	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/pkg/mcp/client"
)

type MockUpdater struct {
	Changes []*client.Change
}

func (mu *MockUpdater) Apply(c *client.Change) error {
	mu.Changes = append(mu.Changes, c)
	return nil
}

var _ = Describe("MCP", func() {
	var (
		session          *gexec.Session
		listenAddrForMCP string
		cleanupFuncs     []func()
	)

	BeforeEach(func() {
		copilotCreds := testhelpers.GenerateMTLS()

		listenAddrForPilot := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
		listenAddrForCloudController := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
		listenAddrForMCP = fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())

		copilotTLSFiles := copilotCreds.CreateServerTLSFiles()
		configFilePath := testhelpers.TempFileName()
		cleanupFuncs = append(cleanupFuncs, func() { os.Remove(configFilePath) })
		cleanupFuncs = append(cleanupFuncs, copilotCreds.CleanupTempFiles)

		bbsCreds := testhelpers.GenerateMTLS()
		cleanupFuncs = append(cleanupFuncs, copilotCreds.CleanupTempFiles)

		bbsTLSFiles := bbsCreds.CreateClientTLSFiles()

		bbsServer := ghttp.NewUnstartedServer()
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
				},
			}
			data, _ := proto.Marshal(&actualLRPResponse)
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		})

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

		bbsServer.Start()
		cleanupFuncs = append(cleanupFuncs, bbsServer.Close)

		serverConfig := &config.Config{
			ListenAddressForPilot:           listenAddrForPilot,
			ListenAddressForCloudController: listenAddrForCloudController,
			ListenAddressForMCP:             listenAddrForMCP,
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

		Expect(serverConfig.Save(configFilePath)).To(Succeed())

		cmd := exec.Command(binaryPath, "-config", configFilePath)
		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session.Out).Should(gbytes.Say(`started`))

		ccClient, err := copilot.NewCloudControllerClient(serverConfig.ListenAddressForCloudController, copilotCreds.OtherClientTLSConfig())
		Expect(err).NotTo(HaveOccurred())

		serverForCloudControllerIsHealthy := func() error {
			ctx, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancelFunc()
			_, err := ccClient.Health(ctx, new(api.HealthRequest))
			return err
		}
		Eventually(serverForCloudControllerIsHealthy, 2*time.Second).Should(Succeed())

		_, err = ccClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
			Route: &api.Route{
				Guid: "route-guid-a",
				Host: "some-url",
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
				CapiProcessGuid: "capi-process-guid-a",
				DiegoProcessGuids: []string{
					"diego-process-guid-a",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		session.Interrupt()
		Eventually(session, "10s").Should(gexec.Exit())

		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	})

	It("sends config over the wire", func() {
		opts := []grpc.DialOption{
			grpc.WithBlock(),
			grpc.WithInsecure(),
			grpc.WithTimeout(5 * time.Second),
		}

		conn, err := grpc.Dial(listenAddrForMCP, opts...)
		Expect(err).NotTo(HaveOccurred())
		defer conn.Close()

		svcClient := mcp.NewAggregatedMeshConfigServiceClient(conn)
		mockUpdater := &MockUpdater{}

		client.New(svcClient, []string{"destination-rule", "virtual-service"}, mockUpdater, "test-id", nil)
		Expect(mockUpdater.Changes).To(HaveLen(2))

		var messageNames []string
		for _, c := range mockUpdater.Changes {
			messageNames = append(messageNames, c.MessageName)
		}

		Expect(messageNames).To(ConsistOf([]string{"virtual-service", "destination-rule"}))
	})
})
