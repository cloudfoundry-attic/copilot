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
		session         *gexec.Session
		istioClient     copilot.IstioClient
		ccClient        copilot.CloudControllerClient
		serverConfig    *config.Config
		clientTLSConfig *tls.Config
		configFilePath  string

		bbsServer    *ghttp.Server
		cleanupFuncs []func()
	)

	BeforeEach(func() {
		copilotCreds := testhelpers.GenerateMTLS()
		cleanupFuncs = append(cleanupFuncs, copilotCreds.CleanupTempFiles)

		listenAddr := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
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
					&bbsmodels.ActualLRPGroup{
						Instance: &bbsmodels.ActualLRP{
							ActualLRPKey: bbsmodels.NewActualLRPKey("process-guid-a", 1, "domain1"),
							State:        bbsmodels.ActualLRPStateRunning,
							ActualLRPNetInfo: bbsmodels.ActualLRPNetInfo{
								Address: "10.10.1.5",
								Ports: []*bbsmodels.PortMapping{
									&bbsmodels.PortMapping{ContainerPort: 8080, HostPort: 61005},
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
			ListenAddress:  listenAddr,
			ClientCAPath:   copilotTLSFiles.ClientCA,
			ServerCertPath: copilotTLSFiles.ServerCert,
			ServerKeyPath:  copilotTLSFiles.ServerKey,
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

		clientTLSConfig = copilotCreds.ClientTLSConfig()

		istioClient, err = copilot.NewIstioClient(serverConfig.ListenAddress, clientTLSConfig)
		Expect(err).NotTo(HaveOccurred())
		ccClient, err = copilot.NewCloudControllerClient(serverConfig.ListenAddress, clientTLSConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		session.Interrupt()
		Eventually(session, "2s").Should(gexec.Exit())

		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	})

	It("serves routes, using data from the BBS", func() {
		WaitForHealthy(istioClient)
		routes, err := istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		Expect(routes.Backends).To(Equal(map[string]*api.BackendSet{
			"process-guid-a.cfapps.internal": &api.BackendSet{
				Backends: []*api.Backend{
					&api.Backend{Address: "10.10.1.5", Port: 61005},
				},
			},
		}))
	})

	It("adds routes", func() {
		WaitForHealthy(istioClient)
		_, err := ccClient.AddRoute(context.Background(), &api.AddRequest{
			ProcessGuid: "process-guid-a",
			Hostname:    "example.route.com",
		})
		Expect(err).NotTo(HaveOccurred())

		routes, err := istioClient.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		Expect(routes.Backends).To(Equal(map[string]*api.BackendSet{
			"process-guid-a.cfapps.internal": &api.BackendSet{
				Backends: []*api.Backend{
					&api.Backend{Address: "10.10.1.5", Port: 61005},
				},
			},
			"example.route.com": &api.BackendSet{
				Backends: []*api.Backend{
					&api.Backend{Address: "10.10.1.5", Port: 61005},
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
		WaitForHealthy(istioClient)
		Consistently(session, "1s").ShouldNot(gexec.Exit())
		_, err := istioClient.Health(context.Background(), new(api.HealthRequest))
		Expect(err).NotTo(HaveOccurred())

		Expect(istioClient.Close()).To(Succeed())
		session.Interrupt()

		Eventually(session, "2s").Should(gexec.Exit())
	})

	Context("when the tls config is invalid", func() {
		BeforeEach(func() {
			clientTLSConfig.RootCAs = nil
			var err error
			istioClient, err = copilot.NewIstioClient(serverConfig.ListenAddress, clientTLSConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		Specify("the istioClient gets a meaningful error", func() {
			_, err := istioClient.Health(context.Background(), new(api.HealthRequest))
			Expect(err).To(MatchError(ContainSubstring("authentication handshake failed")))
		})
	})
})

func WaitForHealthy(istioClient copilot.IstioClient) {
	By("waiting for the server become healthy")
	isHealthy := func() error {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelFunc()
		_, err := istioClient.Health(ctx, new(api.HealthRequest))
		return err
	}
	Eventually(isHealthy, 2*time.Second).Should(Succeed())
}
