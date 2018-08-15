package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

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
				Disable: true,
			},
		}

		Expect(serverConfig.Save(configFilePath)).To(Succeed())

		cmd := exec.Command(binaryPath, "-config", configFilePath)
		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session.Out).Should(gbytes.Say(`started`))
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
		Expect(mockUpdater.Changes).To(HaveLen(1))
	})
})
