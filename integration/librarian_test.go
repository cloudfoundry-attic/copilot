package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"code.cloudfoundry.org/copilot/certs"
	librarian "code.cloudfoundry.org/copilot/librarianconfig"
	"code.cloudfoundry.org/copilot/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Librarian", func() {
	var (
		session        *gexec.Session
		serverConfig   *librarian.Config
		configFilePath string

		cleanupFuncs []func()
	)

	It("stows certificates in the correct place", func() {
		binPath, err := gexec.Build("code.cloudfoundry.org/copilot/cmd/librarian", "-race")
		Expect(err).NotTo(HaveOccurred())
		tempDir, err := ioutil.TempDir("", "certs")
		Expect(err).NotTo(HaveOccurred())

		serverConfig = &librarian.Config{
			IstioCertRootPath: tempDir,
			TLSPems: []certs.CertChainKeyPair{
				{
					CertChain:  "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIRAPf9lECQDqNwfP1KpPxMqmIwDQYJKoZIhvcNAQELBQAw\nEjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xODExMTQxODU0MThaFw0xOTExMTQxODU0\nMThaMBIxEDAOBgNVBAoTB0FjbWUgQ28wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw\nggEKAoIBAQCgDRgg0xFS7Hw4yN/EMTVYp9+My7R6mZ/s7qYVEcrKVnLYZAAUsXsA\nLG1BVeTWfQSPvshi1EP4SAsRpZ8sO/o3GybVfm5ejBVOC0seA1zm2LHMwPyjeIXU\neM/7S3VdBkve+37vj78uZe149Jj+IkLL3zkfRtI+coG9mw4FpP0TqaRQ41cKqnQS\nD2iRbSfBW/nMRcFQr7aK+z+LQg6LPez7CxCsdXgcMf8kNVdceQSatEFnufnK/Gyy\nDs+P2ovlqLpVC05SsO/dTQp+QtVYMNeCA/eLixNzwfiCXhDZ993JFUWj3TkCr7f6\nBY5U/2naXAGS8ZZVzXlweX2SO0BYicPNAgMBAAGjTTBLMA4GA1UdDwEB/wQEAwIF\noDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMBYGA1UdEQQPMA2C\nC2V4YW1wbGUuY29tMA0GCSqGSIb3DQEBCwUAA4IBAQAbgykDDrDA00rKNx/B4G2j\nAeDAHkAnMK5IjdrgH2KUNeI07eRkLhYobrquwcKRYa9RJcM/eImX8BkviwjlOkDz\noJdU0LMVrsrjBuwj9qYg+D7IywvPrrrdrjgF05BxUfwoH1lTKm8Q9SVnpEWEdJj9\n+sP10reX+O7L4xiqgyuKHjWPEK4NJD+Wsw5n8UvIq5LVvTt3bLWsgN2Mole0lJb4\nvgR4N1absHZN6/yju5s7cY0lLBcEitJNUQeW3lHSOWXJ8xiw9aayFnJ4tmNWpALU\nvqettFbN38gfsH8JElHwyeKLthGL/Kj1Cvb//SbK30RnG8vY8kuqKKs3iwuHB+UU\n-----END CERTIFICATE-----\n",
					PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAoA0YINMRUux8OMjfxDE1WKffjMu0epmf7O6mFRHKylZy2GQA\nFLF7ACxtQVXk1n0Ej77IYtRD+EgLEaWfLDv6Nxsm1X5uXowVTgtLHgNc5tixzMD8\no3iF1HjP+0t1XQZL3vt+74+/LmXtePSY/iJCy985H0bSPnKBvZsOBaT9E6mkUONX\nCqp0Eg9okW0nwVv5zEXBUK+2ivs/i0IOiz3s+wsQrHV4HDH/JDVXXHkEmrRBZ7n5\nyvxssg7Pj9qL5ai6VQtOUrDv3U0KfkLVWDDXggP3i4sTc8H4gl4Q2ffdyRVFo905\nAq+3+gWOVP9p2lwBkvGWVc15cHl9kjtAWInDzQIDAQABAoIBABUV5InehLfCBBOP\nEzvLp9WIOEFaTOqh9pnGTwcTkv3ZKcQsWH5ha2z4bWRgJofDbKhrYAb1JAc/poWq\npi+zryE3aIRT5cJ6/guMHVdU5hZbkgEBo8b9h9QYHn5i0JFy1OgJhg2ViIBaWVDI\nGKfSZ65oOCRQtj4X49PQ66X+uICwcWhJ3tZnFVODPQU6uDaUZsJzESTaEYaTEkpH\nKCbYdKL4dqt76SIxzKwy1tQlV7R/5Vl5iGhIq143iqNVEAHnCDzJZyonoFpvzT3A\nKfxYjwbatzDdDDujlzyUEwdzy+ZSkMtb/b2Asd0QseY4LgsjnkyQKTtuemjxLw7F\nrMbD3ZkCgYEAx5hWLceS1li3h4UVQFPEdqLAyBW5xTGAVKP4dEirlDWWlkVNpPSw\nD/ZAMieL7WT40JExsGYovrtly9BgyOkTbhbs3dlTDsd2++2/gycjEiNIw08Q5F0v\nz0TgV5psUb1E2Mvubf+Ns04C/NwXHX+A8ClcHuVy/qw/y92s+r+H2wcCgYEAzUf2\nFu4d+CO2JqcvPY2YikDFNT6pIzO/Ux0W41FwJVDHRa+42vqW5qPrr6ThWoVEEs5h\nzeBgh0X6K+2AbELDm3kxW43ceHo6KmPCPyQcMxff+A8LyxZWxn/8wb4CKso2zc1L\ncm6w5E0NsCmt/4WP5EeIIUmUXIpcNP9uNCZ6sYsCgYAn1q802gXkBLc1NIoGWfH3\n4ApspXF7+6JqwoO/6hVdMskI23Jg/3n45aTwndYfHy1Oq/xoAiwVzd/Gq6P11hfL\nvIWwzkT2yTdll5HHQtOMNkC6wxhTDIqTa2L/+VGviwCn6SSBDiYhaOvNvrxaZe29\ngfPiMtgeHxFoxqlVL0+VlwKBgApa+PULKgPceVHV2TI3tFw1DD21XX7jG2Gr8/2f\nnBKl0oeXZ7HUNkyINFl17dBNLLPuKUzjZrssMoSIxJOxgoCTSoQd0eNZ9xkwUxow\nTiPdrnSq/aNPCy2UQ0Have0+qikTlBy/rLi3klsynw5mxG11lk5nkc5hRGmAASUs\nU8AlAoGAVMzVMvNOC5Q3uiji0HRnZRa9XrOFdLGZjIUqtLAEdCyGG2Q1WBy2aVgX\nHb/NjnkfmroOSCKUOyqFt0N3sHAv65E5rUdY46uyfczyaQ4wjEhxHPCID6aQc/4f\npBr58YgMa/6k4d3H6arh4cXXPZ16r2gxOcwrVeHecxGpfSAtzBg=\n-----END RSA PRIVATE KEY-----\n",
				},
			},
		}

		configFilePath = testhelpers.TempFileName()
		cleanupFuncs = append(cleanupFuncs, func() { os.Remove(configFilePath) })

		Expect(serverConfig.Save(configFilePath)).To(Succeed())

		cmd := exec.Command(binPath, "-config", configFilePath)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session.Out).Should(gbytes.Say(`stowing certs`))
		Eventually(session.Out).Should(gbytes.Say(`certs stowed`))

		By("the certs exist at the correct locations")

		cert, err := ioutil.ReadFile(fmt.Sprintf("%s/example.com/tls.crt", tempDir))
		Expect(string(cert)).To(Equal(serverConfig.TLSPems[0].CertChain))
		key, err := ioutil.ReadFile(fmt.Sprintf("%s/example.com/tls.key", tempDir))
		Expect(string(key)).To(Equal(serverConfig.TLSPems[0].PrivateKey))
	})

	AfterEach(func() {
		session.Interrupt()
		Eventually(session, "10s").Should(gexec.Exit())

		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	})
})
