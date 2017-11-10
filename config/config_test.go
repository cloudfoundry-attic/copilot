package config_test

import (
	"crypto/tls"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var configFile string
	BeforeEach(func() {
		tempFile, err := ioutil.TempFile("", "cfg")
		Expect(err).NotTo(HaveOccurred())
		Expect(tempFile.Close()).To(Succeed())
		configFile = tempFile.Name()
	})

	AfterEach(func() {
		_ = os.Remove(configFile)
	})

	It("saves and loads via JSON", func() {
		originalCfg := &config.Config{
			ClientCA:   "some-ca",
			ServerCert: "some-cert",
			ServerKey:  "some-key",
		}
		err := originalCfg.Save(configFile)
		Expect(err).NotTo(HaveOccurred())

		loadedCfg, err := config.Load(configFile)
		Expect(err).NotTo(HaveOccurred())

		Expect(loadedCfg).To(Equal(originalCfg))
	})

	Context("when the file is not valid json", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(configFile, []byte("nope"), 0600)).To(Succeed())
		})

		It("returns a meaningful error", func() {
			_, err := config.Load(configFile)
			Expect(err).To(MatchError(HavePrefix("parsing config: invalid")))
		})
	})

	Describe("building the server TLS config", func() {
		var rawConfig config.Config

		BeforeEach(func() {
			clientCreds := testhelpers.GenerateCredentials("clientCA", "client")
			serverCreds := testhelpers.GenerateCredentials("serverCA", "CopilotServer")
			rawConfig = config.Config{
				ClientCA:   string(clientCreds.CA),
				ServerCert: string(serverCreds.Cert),
				ServerKey:  string(serverCreds.Key),
			}
		})

		It("returns a valid tls.Config", func() {
			tlsConfig, err := rawConfig.ServerTLSConfig()
			Expect(err).NotTo(HaveOccurred())

			ln, err := tls.Listen("tcp", ":", tlsConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(ln).NotTo(BeNil())
		})

		It("sets secure values for configuration parameters", func() {
			tlsConfig, err := rawConfig.ServerTLSConfig()
			Expect(err).NotTo(HaveOccurred())

			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConfig.PreferServerCipherSuites).To(BeTrue())
			Expect(tlsConfig.CipherSuites).To(ConsistOf([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			}))
			Expect(tlsConfig.CurvePreferences).To(ConsistOf([]tls.CurveID{
				tls.CurveP384,
			}))
			Expect(tlsConfig.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
			Expect(tlsConfig.ClientCAs).ToNot(BeNil())
			Expect(tlsConfig.ClientCAs.Subjects()).To(ConsistOf(ContainSubstring("clientCA")))
		})

		Context("when the client CA PEM data is invalid", func() {
			BeforeEach(func() {
				rawConfig.ClientCA = "invalid pem"
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfig()
				Expect(err).To(MatchError("parsing client CAs: invalid pem block"))
			})
		})

		Context("when the server cert PEM data is invalid", func() {
			BeforeEach(func() {
				rawConfig.ServerCert = "invalid pem"
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfig()
				Expect(err).To(MatchError(HavePrefix("parsing server cert/key: tls: failed to find any PEM data")))
			})
		})

		Context("when the server key PEM data is invalid", func() {
			BeforeEach(func() {
				rawConfig.ServerKey = "invalid pem"
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfig()
				Expect(err).To(MatchError(HavePrefix("parsing server cert/key: tls: failed to find any PEM data")))
			})
		})

	})
})
