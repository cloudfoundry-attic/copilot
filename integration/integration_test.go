package integration_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"os/exec"
	"time"

	"code.cloudfoundry.org/copilot"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Copilot", func() {
	var (
		session         *gexec.Session
		client          copilot.Client
		serverConfig    *config.Config
		clientTLSConfig *tls.Config
	)

	BeforeEach(func() {
		creds := testhelpers.GenerateMTLS()
		listenAddr := fmt.Sprintf("127.0.0.1:%d", testhelpers.PickAPort())

		serverConfig = &config.Config{
			ListenAddress: listenAddr,
			ClientCA:      string(creds.Client.CA),
			ServerCert:    string(creds.Server.Cert),
			ServerKey:     string(creds.Server.Key),
		}
		configFilePath := testhelpers.TempFileName()
		Expect(serverConfig.Save(configFilePath)).To(Succeed())

		cmd := exec.Command(binaryPath, "-config", configFilePath)
		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session.Out).Should(gbytes.Say(`started`))

		clientTLSConfig = creds.ClientTLSConfig()

		client, err = copilot.NewClient(serverConfig.ListenAddress, clientTLSConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		session.Interrupt()
		Eventually(session, "2s").Should(gexec.Exit())
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
