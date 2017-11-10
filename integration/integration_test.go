package integration_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os/exec"
	"time"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const DEFAULT_TIMEOUT = 2 * time.Second

func StartAndWaitForServer(binaryPath string) *gexec.Session {
	serverCreds := testhelpers.GenerateCredentials("serverCA", "CopilotServer")
	clientCreds := testhelpers.GenerateCredentials("clientCA", "CopilotClient")

	cfg := &config.Config{
		ClientCA:   string(clientCreds.CA),
		ServerCert: string(serverCreds.Cert),
		ServerKey:  string(serverCreds.Key),
	}
	cfgFile, err := ioutil.TempFile("", "test-config")
	Expect(err).NotTo(HaveOccurred())
	Expect(cfgFile.Close()).To(Succeed())
	configFilePath := cfgFile.Name()

	Expect(cfg.Save(configFilePath)).To(Succeed())

	cmd := exec.Command(binaryPath, "-config", configFilePath)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	rootCAs := x509.NewCertPool()
	ok := rootCAs.AppendCertsFromPEM(serverCreds.CA)
	Expect(ok).To(BeTrue())

	clientCert, err := tls.X509KeyPair(clientCreds.Cert, clientCreds.Key)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session.Out).Should(gbytes.Say(`Copilot started`))

	By("waiting for the server to boot")
	serverIsUp := func() error {
		tlsConfig := &tls.Config{
			RootCAs:      rootCAs,
			Certificates: []tls.Certificate{clientCert},
		}

		conn, err := grpc.Dial("127.0.0.1:8888",
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
			grpc.WithTimeout(DEFAULT_TIMEOUT),
			grpc.WithBlock(),
		)
		if err != nil {
			return fmt.Errorf("Dial error: %+v", err)
		}
		defer conn.Close()

		client := api.NewCopilotClient(conn)
		ctx, cancelFunc := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
		defer cancelFunc()
		_, err = client.Health(ctx, new(api.HealthRequest))
		if err != nil {
			return fmt.Errorf("Health error: %s", err)
		}
		return nil
	}
	Eventually(serverIsUp, DEFAULT_TIMEOUT).Should(Succeed())
	return session
}

var _ = Describe("Copilot", func() {
	var session *gexec.Session
	BeforeEach(func() {
		session = StartAndWaitForServer(binaryPath)
	})

	AfterEach(func() {
		session.Interrupt()
		Eventually(session, DEFAULT_TIMEOUT).Should(gexec.Exit())
	})

	It("gracefully terminates when sent an interrupt signal", func() {
		Consistently(session, "1s").ShouldNot(gexec.Exit())

		session.Interrupt()

		Eventually(session, DEFAULT_TIMEOUT).Should(gexec.Exit())
	})
})
