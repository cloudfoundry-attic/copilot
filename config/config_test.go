package config_test

import (
	"crypto/tls"
	"io/ioutil"
	"os"
	"reflect"

	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		configFile string
		cfg        *config.Config
	)

	BeforeEach(func() {
		configFile = testhelpers.TempFileName()
		cfg = &config.Config{
			ListenAddress:  "127.0.0.1:1234",
			ClientCAPath:   "some-ca-path",
			ServerCertPath: "some-cert-path",
			ServerKeyPath:  "some-key-path",
			BBS: config.BBSConfig{
				ServerCACertPath: "some-ca-path",
				ClientCertPath:   "some-cert-path",
				ClientKeyPath:    "some-key-path",
				Address:          "127.0.0.1:8889",
			},
		}
	})

	AfterEach(func() {
		_ = os.Remove(configFile)
	})

	It("saves and loads via JSON", func() {
		err := cfg.Save(configFile)
		Expect(err).NotTo(HaveOccurred())

		loadedCfg, err := config.Load(configFile)
		Expect(err).NotTo(HaveOccurred())

		Expect(loadedCfg).To(Equal(cfg))
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

	DescribeTable("required fields",
		func(fieldName string) {
			// zero out the named field of the cfg struct
			fieldValue := reflect.Indirect(reflect.ValueOf(cfg)).FieldByName(fieldName)
			fieldValue.Set(reflect.Zero(fieldValue.Type()))

			// save to the file
			Expect(cfg.Save(configFile)).To(Succeed())
			// attempt to load it
			_, err := config.Load(configFile)
			Expect(err).To(MatchError(HavePrefix("invalid config: " + fieldName)))
		},
		Entry("ListenAddress", "ListenAddress"),
		Entry("ClientCAPath", "ClientCAPath"),
		Entry("ServerCertPath", "ServerCertPath"),
		Entry("ServerKeyPath", "ServerKeyPath"),
	)

	DescribeTable("required BBS fields",
		func(fieldName string) {
			// zero out the named field of the cfg struct
			fieldValue := reflect.Indirect(reflect.ValueOf(&cfg.BBS)).FieldByName(fieldName)
			fieldValue.Set(reflect.Zero(fieldValue.Type()))

			// save to the file
			Expect(cfg.Save(configFile)).To(Succeed())
			// attempt to load it
			_, err := config.Load(configFile)
			Expect(err).To(MatchError(HavePrefix("invalid config: BBS." + fieldName)))
		},
		Entry("ServerCACertPath", "ServerCACertPath"),
		Entry("ClientCertPath", "ClientCertPath"),
		Entry("ClientKeyPath", "ClientKeyPath"),
		Entry("Address", "Address"),
	)

	Describe("building the server TLS config", func() {
		var rawConfig config.Config

		BeforeEach(func() {
			creds := testhelpers.GenerateMTLS()
			tlsFiles := creds.CreateServerTLSFiles()

			rawConfig = config.Config{
				ClientCAPath:   tlsFiles.ClientCA,
				ServerCertPath: tlsFiles.ServerCert,
				ServerKeyPath:  tlsFiles.ServerKey,
			}
		})

		AfterEach(func() {
			_ = os.Remove(rawConfig.ClientCAPath)
			_ = os.Remove(rawConfig.ServerCertPath)
			_ = os.Remove(rawConfig.ServerKeyPath)
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

		Context("when the client CA file does not exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(rawConfig.ClientCAPath)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfig()
				Expect(err).To(MatchError(HavePrefix("loading client CAs: open")))
			})
		})
		Context("when the client CA PEM data is invalid", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(rawConfig.ClientCAPath, []byte("invalid pem"), 0600)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfig()
				Expect(err).To(MatchError("parsing client CAs: invalid pem block"))
			})
		})

		Context("when the server cert PEM data is invalid", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(rawConfig.ServerCertPath, []byte("invalid pem"), 0600)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfig()
				Expect(err).To(MatchError(HavePrefix("parsing server cert/key: tls: failed to find any PEM data")))
			})
		})

		Context("when the server key PEM data is invalid", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(rawConfig.ServerKeyPath, []byte("invalid pem"), 0600)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfig()
				Expect(err).To(MatchError(HavePrefix("parsing server cert/key: tls: failed to find any PEM data")))
			})
		})
	})
})
