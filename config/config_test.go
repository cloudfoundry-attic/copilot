package config_test

import (
	"crypto/tls"
	"io/ioutil"
	"os"
	"reflect"
	"time"

	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	const configFilePath = "./config_test.json"
	var (
		configFile string
		cfg        *config.Config
	)

	BeforeEach(func() {
		configFile = testhelpers.TempFileName()
		cfg = &config.Config{
			ListenAddressForPilot:           "127.0.0.1:1234",
			ListenAddressForCloudController: "127.0.0.1:1235",
			ListenAddressForMCP:             "127.0.0.1:1236",
			PilotClientCAPath:               "some-pilot-ca-path",
			CloudControllerClientCAPath:     "some-cloud-controller-ca-path",
			ServerCertPath:                  "some-cert-path",
			ServerKeyPath:                   "some-key-path",
			VIPCIDR:                         "127.128.0.0/9",
			MCPConvergeInterval:             10 * time.Second,
			BBS: &config.BBSConfig{
				ServerCACertPath: "some-ca-path",
				ClientCertPath:   "some-cert-path",
				ClientKeyPath:    "some-key-path",
				Address:          "127.0.0.1:8889",
				SyncInterval:     5 * time.Second,
			},
		}
	})

	AfterEach(func() {
		_ = os.Remove(configFile)
	})

	It("loads and converts a JSON file to a Config", func() {
		loadedCfg, err := config.Load(configFilePath)
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
		Entry("ListenAddressForPilot", "ListenAddressForPilot"),
		Entry("ListenAddressForCloudController", "ListenAddressForCloudController"),
		Entry("ListenAddressForMCP", "ListenAddressForMCP"),
		Entry("PilotClientCAPath", "PilotClientCAPath"),
		Entry("CloudControllerClientCAPath", "CloudControllerClientCAPath"),
		Entry("ServerCertPath", "ServerCertPath"),
		Entry("ServerKeyPath", "ServerKeyPath"),
		Entry("VIPCIDR", "VIPCIDR"),
	)

	DescribeTable("required BBS fields",
		func(fieldName string) {
			// zero out the named field of the cfg struct
			fieldValue := reflect.Indirect(reflect.ValueOf(cfg.BBS)).FieldByName(fieldName)
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

	Describe("optional BBS fields", func() {
		Context("when SyncInterval is provided in the config", func() {
			BeforeEach(func() {
				cfg.BBS.SyncInterval = 10 * time.Second
			})

			It("uses the config's sync interval", func() {
				err := cfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.BBS.SyncInterval).To(Equal(10 * time.Second))
			})
		})

		Context("when SyncInterval is not set or zero", func() {
			BeforeEach(func() {
				cfg.BBS.SyncInterval = 0
			})

			It("defaults to 60 seconds", func() {
				err := cfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.BBS.SyncInterval).To(Equal(60 * time.Second))
			})
		})
	})

	Describe("optional fields", func() {
		Context("when MCPConvergeInterval is provided in the config", func() {
			BeforeEach(func() {
				cfg.MCPConvergeInterval = 7 * time.Second
			})

			It("uses the config's sync interval", func() {
				err := cfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.MCPConvergeInterval).To(Equal(7 * time.Second))
			})
		})

		Context("when MCPConvergeInterval is not set or zero", func() {
			BeforeEach(func() {
				cfg.MCPConvergeInterval = 0
			})

			It("defaults to 30 seconds", func() {
				err := cfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.MCPConvergeInterval).To(Equal(30 * time.Second))
			})
		})
	})

	Context("when BBS.Disable is true but other BBS fields are empty", func() {
		BeforeEach(func() {
			cfg.BBS = &config.BBSConfig{
				Disable: true,
			}
			err := cfg.Save(configFile)
			Expect(err).NotTo(HaveOccurred())
		})
		It("validates ok", func() {
			loadedCfg, err := config.Load(configFile)
			Expect(err).NotTo(HaveOccurred())
			cfg.BBS = nil
			Expect(loadedCfg).To(Equal(cfg))
		})
	})

	Context("when BBS is missing", func() {
		BeforeEach(func() {
			cfg.BBS = nil
			err := cfg.Save(configFile)
			Expect(err).NotTo(HaveOccurred())
		})
		It("fails with a useful error message", func() {
			_, err := config.Load(configFile)
			Expect(err).To(MatchError("invalid config: missing required 'BBS' field"))
		})
	})

	Describe("building the server TLS config", func() {
		var rawConfig config.Config

		BeforeEach(func() {
			creds := testhelpers.GenerateMTLS()
			tlsFiles := creds.CreateServerTLSFiles()

			rawConfig = config.Config{
				PilotClientCAPath:           tlsFiles.ClientCA,
				CloudControllerClientCAPath: tlsFiles.OtherClientCA,
				ServerCertPath:              tlsFiles.ServerCert,
				ServerKeyPath:               tlsFiles.ServerKey,
			}
		})

		AfterEach(func() {
			_ = os.Remove(rawConfig.PilotClientCAPath)
			_ = os.Remove(rawConfig.CloudControllerClientCAPath)
			_ = os.Remove(rawConfig.ServerCertPath)
			_ = os.Remove(rawConfig.ServerKeyPath)
		})

		It("returns a valid tls.Config for the Pilot-facing server", func() {
			tlsConfig, err := rawConfig.ServerTLSConfigForPilot()
			Expect(err).NotTo(HaveOccurred())

			ln, err := tls.Listen("tcp", ":", tlsConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(ln).NotTo(BeNil())
		})

		It("returns a valid tls.Config for the Cloud Controller-facing server", func() {
			tlsConfig, err := rawConfig.ServerTLSConfigForCloudController()
			Expect(err).NotTo(HaveOccurred())

			ln, err := tls.Listen("tcp", ":", tlsConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(ln).NotTo(BeNil())
		})

		It("sets secure values for configuration parameters for the Pilot-facing server", func() {
			tlsConfig, err := rawConfig.ServerTLSConfigForPilot()
			Expect(err).NotTo(HaveOccurred())

			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConfig.PreferServerCipherSuites).To(BeTrue())
			Expect(tlsConfig.CipherSuites).To(ConsistOf([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			}))
			Expect(tlsConfig.CurvePreferences).To(BeEmpty())
			Expect(tlsConfig.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
			Expect(tlsConfig.ClientCAs).ToNot(BeNil())
			Expect(tlsConfig.ClientCAs.Subjects()).To(ConsistOf(ContainSubstring("clientCA")))
		})

		It("sets secure values for configuration parameters for the Cloud Controller-facing server", func() {
			tlsConfig, err := rawConfig.ServerTLSConfigForCloudController()
			Expect(err).NotTo(HaveOccurred())

			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConfig.PreferServerCipherSuites).To(BeTrue())
			Expect(tlsConfig.CipherSuites).To(ConsistOf([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			}))
			Expect(tlsConfig.CurvePreferences).To(BeEmpty())
			Expect(tlsConfig.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
			Expect(tlsConfig.ClientCAs).ToNot(BeNil())
			Expect(tlsConfig.ClientCAs.Subjects()).To(ConsistOf(ContainSubstring("otherClientCA")))
		})

		Context("when the pilot client CA file does not exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(rawConfig.PilotClientCAPath)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfigForPilot()
				Expect(err).To(MatchError(HavePrefix("loading client CAs for pilot-facing server: open")))
			})
		})

		Context("when the cloud controller client CA file does not exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(rawConfig.CloudControllerClientCAPath)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfigForCloudController()
				Expect(err).To(MatchError(HavePrefix("loading client CAs for cloud controller-facing server: open")))
			})
		})

		Context("when the pilot client CA PEM data is invalid", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(rawConfig.PilotClientCAPath, []byte("invalid pem"), 0600)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfigForPilot()
				Expect(err).To(MatchError("parsing client CAs for pilot-facing server: invalid pem block"))
			})
		})

		Context("when the cloud controller client CA PEM data is invalid", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(rawConfig.CloudControllerClientCAPath, []byte("invalid pem"), 0600)).To(Succeed())
			})
			It("returns a meaningful error", func() {
				_, err := rawConfig.ServerTLSConfigForCloudController()
				Expect(err).To(MatchError("parsing client CAs for cloud controller-facing server: invalid pem block"))
			})
		})

		Context("when the server cert PEM data is invalid", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(rawConfig.ServerCertPath, []byte("invalid pem"), 0600)).To(Succeed())
			})
			It("returns a meaningful error when loading the pilot-facing config", func() {
				_, err := rawConfig.ServerTLSConfigForPilot()
				Expect(err).To(MatchError(HavePrefix("parsing pilot-facing server cert/key: tls: failed to find any PEM data")))
			})
			It("returns a meaningful error when loading the cloud controller-facing config", func() {
				_, err := rawConfig.ServerTLSConfigForCloudController()
				Expect(err).To(MatchError(HavePrefix("parsing cloud controller-facing server cert/key: tls: failed to find any PEM data")))
			})
		})

		Context("when the server key PEM data is invalid", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(rawConfig.ServerKeyPath, []byte("invalid pem"), 0600)).To(Succeed())
			})
			It("returns a meaningful error when loading the pilot-facing config", func() {
				_, err := rawConfig.ServerTLSConfigForPilot()
				Expect(err).To(MatchError(HavePrefix("parsing pilot-facing server cert/key: tls: failed to find any PEM data")))
			})
			It("returns a meaningful error when loading the cloud controller-facing config", func() {
				_, err := rawConfig.ServerTLSConfigForCloudController()
				Expect(err).To(MatchError(HavePrefix("parsing cloud controller-facing server cert/key: tls: failed to find any PEM data")))
			})
		})
	})
})
