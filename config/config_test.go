package config_test

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"time"

	"code.cloudfoundry.org/copilot/certs"
	"code.cloudfoundry.org/copilot/config"
	"code.cloudfoundry.org/copilot/testhelpers"
	"code.cloudfoundry.org/durationjson"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	const configFilePath = "./config_test.json"
	var (
		configFile  string
		expectedCfg *config.Config
	)

	BeforeEach(func() {
		configFile = testhelpers.TempFileName()
		expectedCfg = &config.Config{
			ListenAddressForCloudController: "127.0.0.1:1235",
			ListenAddressForMCP:             "127.0.0.1:1236",
			PilotClientCAPath:               "some-pilot-ca-path",
			CloudControllerClientCAPath:     "some-cloud-controller-ca-path",
			ServerCertPath:                  "some-cert-path",
			ServerKeyPath:                   "some-key-path",
			VIPCIDR:                         "127.128.0.0/9",
			MCPConvergeInterval:             durationjson.Duration(10 * time.Second),
			BBS: &config.BBSConfig{
				ServerCACertPath: "some-ca-path",
				ClientCertPath:   "some-cert-path",
				ClientKeyPath:    "some-key-path",
				Address:          "127.0.0.1:8889",
				SyncInterval:     durationjson.Duration(5 * time.Second),
			},
			TLSPems: []certs.CertChainKeyPair{
				{
					CertChain:  "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIRAPf9lECQDqNwfP1KpPxMqmIwDQYJKoZIhvcNAQELBQAw\nEjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xODExMTQxODU0MThaFw0xOTExMTQxODU0\nMThaMBIxEDAOBgNVBAoTB0FjbWUgQ28wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw\nggEKAoIBAQCgDRgg0xFS7Hw4yN/EMTVYp9+My7R6mZ/s7qYVEcrKVnLYZAAUsXsA\nLG1BVeTWfQSPvshi1EP4SAsRpZ8sO/o3GybVfm5ejBVOC0seA1zm2LHMwPyjeIXU\neM/7S3VdBkve+37vj78uZe149Jj+IkLL3zkfRtI+coG9mw4FpP0TqaRQ41cKqnQS\nD2iRbSfBW/nMRcFQr7aK+z+LQg6LPez7CxCsdXgcMf8kNVdceQSatEFnufnK/Gyy\nDs+P2ovlqLpVC05SsO/dTQp+QtVYMNeCA/eLixNzwfiCXhDZ993JFUWj3TkCr7f6\nBY5U/2naXAGS8ZZVzXlweX2SO0BYicPNAgMBAAGjTTBLMA4GA1UdDwEB/wQEAwIF\noDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMBYGA1UdEQQPMA2C\nC2V4YW1wbGUuY29tMA0GCSqGSIb3DQEBCwUAA4IBAQAbgykDDrDA00rKNx/B4G2j\nAeDAHkAnMK5IjdrgH2KUNeI07eRkLhYobrquwcKRYa9RJcM/eImX8BkviwjlOkDz\noJdU0LMVrsrjBuwj9qYg+D7IywvPrrrdrjgF05BxUfwoH1lTKm8Q9SVnpEWEdJj9\n+sP10reX+O7L4xiqgyuKHjWPEK4NJD+Wsw5n8UvIq5LVvTt3bLWsgN2Mole0lJb4\nvgR4N1absHZN6/yju5s7cY0lLBcEitJNUQeW3lHSOWXJ8xiw9aayFnJ4tmNWpALU\nvqettFbN38gfsH8JElHwyeKLthGL/Kj1Cvb//SbK30RnG8vY8kuqKKs3iwuHB+UU\n-----END CERTIFICATE-----\n",
					PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAoA0YINMRUux8OMjfxDE1WKffjMu0epmf7O6mFRHKylZy2GQA\nFLF7ACxtQVXk1n0Ej77IYtRD+EgLEaWfLDv6Nxsm1X5uXowVTgtLHgNc5tixzMD8\no3iF1HjP+0t1XQZL3vt+74+/LmXtePSY/iJCy985H0bSPnKBvZsOBaT9E6mkUONX\nCqp0Eg9okW0nwVv5zEXBUK+2ivs/i0IOiz3s+wsQrHV4HDH/JDVXXHkEmrRBZ7n5\nyvxssg7Pj9qL5ai6VQtOUrDv3U0KfkLVWDDXggP3i4sTc8H4gl4Q2ffdyRVFo905\nAq+3+gWOVP9p2lwBkvGWVc15cHl9kjtAWInDzQIDAQABAoIBABUV5InehLfCBBOP\nEzvLp9WIOEFaTOqh9pnGTwcTkv3ZKcQsWH5ha2z4bWRgJofDbKhrYAb1JAc/poWq\npi+zryE3aIRT5cJ6/guMHVdU5hZbkgEBo8b9h9QYHn5i0JFy1OgJhg2ViIBaWVDI\nGKfSZ65oOCRQtj4X49PQ66X+uICwcWhJ3tZnFVODPQU6uDaUZsJzESTaEYaTEkpH\nKCbYdKL4dqt76SIxzKwy1tQlV7R/5Vl5iGhIq143iqNVEAHnCDzJZyonoFpvzT3A\nKfxYjwbatzDdDDujlzyUEwdzy+ZSkMtb/b2Asd0QseY4LgsjnkyQKTtuemjxLw7F\nrMbD3ZkCgYEAx5hWLceS1li3h4UVQFPEdqLAyBW5xTGAVKP4dEirlDWWlkVNpPSw\nD/ZAMieL7WT40JExsGYovrtly9BgyOkTbhbs3dlTDsd2++2/gycjEiNIw08Q5F0v\nz0TgV5psUb1E2Mvubf+Ns04C/NwXHX+A8ClcHuVy/qw/y92s+r+H2wcCgYEAzUf2\nFu4d+CO2JqcvPY2YikDFNT6pIzO/Ux0W41FwJVDHRa+42vqW5qPrr6ThWoVEEs5h\nzeBgh0X6K+2AbELDm3kxW43ceHo6KmPCPyQcMxff+A8LyxZWxn/8wb4CKso2zc1L\ncm6w5E0NsCmt/4WP5EeIIUmUXIpcNP9uNCZ6sYsCgYAn1q802gXkBLc1NIoGWfH3\n4ApspXF7+6JqwoO/6hVdMskI23Jg/3n45aTwndYfHy1Oq/xoAiwVzd/Gq6P11hfL\nvIWwzkT2yTdll5HHQtOMNkC6wxhTDIqTa2L/+VGviwCn6SSBDiYhaOvNvrxaZe29\ngfPiMtgeHxFoxqlVL0+VlwKBgApa+PULKgPceVHV2TI3tFw1DD21XX7jG2Gr8/2f\nnBKl0oeXZ7HUNkyINFl17dBNLLPuKUzjZrssMoSIxJOxgoCTSoQd0eNZ9xkwUxow\nTiPdrnSq/aNPCy2UQ0Have0+qikTlBy/rLi3klsynw5mxG11lk5nkc5hRGmAASUs\nU8AlAoGAVMzVMvNOC5Q3uiji0HRnZRa9XrOFdLGZjIUqtLAEdCyGG2Q1WBy2aVgX\nHb/NjnkfmroOSCKUOyqFt0N3sHAv65E5rUdY46uyfczyaQ4wjEhxHPCID6aQc/4f\npBr58YgMa/6k4d3H6arh4cXXPZ16r2gxOcwrVeHecxGpfSAtzBg=\n-----END RSA PRIVATE KEY-----\n",
				},
			},
		}
	})

	AfterEach(func() {
		_ = os.Remove(configFile)
	})

	It("loads and converts a JSON file to a Config", func() {
		loadedCfg, err := config.Load(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(loadedCfg).To(Equal(expectedCfg))
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
			// zero out the named field of the expectedCfg struct
			fieldValue := reflect.Indirect(reflect.ValueOf(expectedCfg)).FieldByName(fieldName)
			fieldValue.Set(reflect.Zero(fieldValue.Type()))

			// save to the file
			Expect(expectedCfg.Save(configFile)).To(Succeed())
			// attempt to load it
			_, err := config.Load(configFile)
			Expect(err).To(MatchError(HavePrefix("invalid config: " + fieldName)))
		},
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
			// zero out the named field of the expectedCfg struct
			fieldValue := reflect.Indirect(reflect.ValueOf(expectedCfg.BBS)).FieldByName(fieldName)
			fieldValue.Set(reflect.Zero(fieldValue.Type()))

			// save to the file
			Expect(expectedCfg.Save(configFile)).To(Succeed())
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
				expectedCfg.BBS.SyncInterval = durationjson.Duration(10 * time.Second)
			})

			It("uses the config's sync interval", func() {
				err := expectedCfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.BBS.SyncInterval).To(Equal(durationjson.Duration(10 * time.Second)))
			})
		})

		Context("when SyncInterval is not set or zero", func() {
			BeforeEach(func() {
				expectedCfg.BBS.SyncInterval = 0
			})

			It("defaults to 60 seconds", func() {
				err := expectedCfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.BBS.SyncInterval).To(Equal(durationjson.Duration(60 * time.Second)))
			})
		})
	})

	Describe("optional fields", func() {
		Context("when MCPConvergeInterval is provided in the config", func() {
			BeforeEach(func() {
				expectedCfg.MCPConvergeInterval = durationjson.Duration(7 * time.Second)
			})

			It("uses the config's sync interval", func() {
				err := expectedCfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.MCPConvergeInterval).To(Equal(durationjson.Duration(7 * time.Second)))
			})
		})

		Context("when MCPConvergeInterval is not set or zero", func() {
			BeforeEach(func() {
				expectedCfg.MCPConvergeInterval = 0
			})

			It("defaults to 30 seconds", func() {
				err := expectedCfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())

				loadedCfg, err := config.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(loadedCfg.MCPConvergeInterval).To(Equal(durationjson.Duration(30 * time.Second)))
			})
		})
	})

	Context("when BBS.Disable is true but other BBS fields are empty", func() {
		BeforeEach(func() {
			expectedCfg.BBS = &config.BBSConfig{
				Disable: true,
			}
			err := expectedCfg.Save(configFile)
			Expect(err).NotTo(HaveOccurred())
		})
		It("validates ok", func() {
			loadedCfg, err := config.Load(configFile)
			Expect(err).NotTo(HaveOccurred())
			expectedCfg.BBS = nil
			Expect(loadedCfg).To(Equal(expectedCfg))
		})
	})

	Context("when BBS is missing", func() {
		BeforeEach(func() {
			expectedCfg.BBS = nil
			err := expectedCfg.Save(configFile)
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

	Describe("building the vip cidr", func() {
		It("returns a cidr object", func() {
			_, cidr, err := net.ParseCIDR("127.128.0.0/9")
			Expect(err).NotTo(HaveOccurred())

			vipCIDR, err := expectedCfg.GetVIPCIDR()
			Expect(err).NotTo(HaveOccurred())
			Expect(vipCIDR).To(Equal(cidr))
		})

		Context("when the provided CIDR cannot be parsed", func() {
			BeforeEach(func() {
				expectedCfg.VIPCIDR = "12.12.12.12.12/7"
				err := expectedCfg.Save(configFile)
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns a CIDR parsing error", func() {
				_, err := config.Load(configFile)
				Expect(err).To(MatchError(HavePrefix("invalid config: VIPCIDR: invalid CIDR address: 12.12.12.12.12/7")))
			})
		})
	})
})
