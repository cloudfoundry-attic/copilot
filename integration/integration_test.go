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
		client          copilot.Client
		serverConfig    *config.Config
		clientTLSConfig *tls.Config
		configFilePath  string

		bbsServer *ghttp.Server
	)

	BeforeEach(func() {
		copilotCreds := testhelpers.GenerateMTLS()
		listenAddr := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())
		copilotTLSFiles := copilotCreds.CreateServerTLSFiles()

		bbsCreds := testhelpers.GenerateMTLS()
		bbsTLSFiles := bbsCreds.CreateClientTLSFiles()

		// boot a fake BBS
		bbsServer = ghttp.NewUnstartedServer()
		bbsServer.HTTPTestServer.TLS = bbsCreds.ServerTLSConfig()

		bbsServer.RouteToHandler("POST", "/v1/actual_lrp_groups/list",
			func(w http.ResponseWriter, req *http.Request) {
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
		Expect(serverConfig.Save(configFilePath)).To(Succeed())

		cmd := exec.Command(binaryPath, "-config", configFilePath)
		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session.Out).Should(gbytes.Say(`started`))

		clientTLSConfig = copilotCreds.ClientTLSConfig()

		client, err = copilot.NewClient(serverConfig.ListenAddress, clientTLSConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		session.Interrupt()
		Eventually(session, "2s").Should(gexec.Exit())

		bbsServer.Close()
		_ = os.Remove(configFilePath)
		_ = os.Remove(serverConfig.ClientCAPath)
		_ = os.Remove(serverConfig.ServerCertPath)
		_ = os.Remove(serverConfig.ServerKeyPath)
	})

	It("serves routes, using data from the BBS", func() {
		WaitForHealthy(client)
		routes, err := client.Routes(context.Background(), new(api.RoutesRequest))
		Expect(err).NotTo(HaveOccurred())
		Expect(routes.Backends).To(Equal(map[string]*api.BackendSet{
			"process-guid-a.internal.tld": &api.BackendSet{
				Backends: []*api.Backend{
					&api.Backend{Address: "10.10.1.5", Port: 61005},
				},
			},
		}))
	})

	It("gracefully terminates when sent an interrupt signal", func() {
		WaitForHealthy(client)
		Consistently(session, "1s").ShouldNot(gexec.Exit())
		_, err := client.Health(context.Background(), new(api.HealthRequest))
		Expect(err).NotTo(HaveOccurred())

		Expect(client.Close()).To(Succeed())
		session.Interrupt()

		Eventually(session, "2s").Should(gexec.Exit())
	})

	Context("when the tls config is invalid", func() {
		BeforeEach(func() {
			clientTLSConfig.RootCAs = nil
			var err error
			client, err = copilot.NewClient(serverConfig.ListenAddress, clientTLSConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		Specify("the client gets a meaningful error", func() {
			_, err := client.Health(context.Background(), new(api.HealthRequest))
			Expect(err).To(MatchError(ContainSubstring("authentication handshake failed")))
		})
	})
})

func WaitForHealthy(client copilot.Client) {
	By("waiting for the server become healthy")
	isHealthy := func() error {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelFunc()
		_, err := client.Health(ctx, new(api.HealthRequest))
		return err
	}
	Eventually(isHealthy, 2*time.Second).Should(Succeed())
}
