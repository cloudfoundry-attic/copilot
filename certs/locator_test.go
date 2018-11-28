package certs_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/copilot/certs"
)

var _ = Describe("Locator", func() {
	var _ = Describe("Locate", func() {
		It("returns cert and key paths and their associated hostnames", func() {
			dnsNames := []string{"example.com"}
			certChain := createPEMSforCertChain(dnsNames)
			pairs := []certs.CertChainKeyPair{
				{
					CertChain: certChain.EndUserCert,
				},
			}

			locator := certs.NewLocator("/some/path", pairs)
			expectedPaths := certs.PemInfo{
				Hosts:    []string{"example.com"},
				CertPath: "/some/path/example.com/tls.crt",
				KeyPath:  "/some/path/example.com/tls.key",
				Dir:      "/some/path/example.com",
			}

			paths, err := locator.Locate()

			Expect(err).NotTo(HaveOccurred())
			Expect(paths).To(ConsistOf(expectedPaths))
		})

		Context("when multiple hostnames are provided for a cert chain and key pair", func() {
			It("returns cert and key paths and their associated hostnames with the first hostname used in path", func() {
				dnsNames := []string{"example.com", "example2.com", "example3.com"}
				certChain := createPEMSforCertChain(dnsNames)
				pairs := []certs.CertChainKeyPair{
					{
						CertChain: certChain.EndUserCert,
					},
				}

				locator := certs.NewLocator("/some/path", pairs)
				expectedPaths := certs.PemInfo{
					Hosts:    []string{"example.com", "example2.com", "example3.com"},
					CertPath: "/some/path/example.com/tls.crt",
					KeyPath:  "/some/path/example.com/tls.key",
					Dir:      "/some/path/example.com",
				}

				paths, err := locator.Locate()

				Expect(err).NotTo(HaveOccurred())
				Expect(paths).To(ConsistOf(expectedPaths))
			})
		})

		Context("When a certchain provided", func() {
			It("returns cert and key paths and their associated hostnames", func() {
				dnsNames := []string{"example.com", "example2.com"}
				certChain := createPEMSforCertChain(dnsNames)
				pairs := []certs.CertChainKeyPair{
					{
						CertChain: fmt.Sprintf("%s%s", certChain.EndUserCert, certChain.RootCert),
					},
				}

				locator := certs.NewLocator("/some/path", pairs)
				expectedPaths := certs.PemInfo{
					Hosts:    dnsNames,
					CertPath: "/some/path/example.com/tls.crt",
					KeyPath:  "/some/path/example.com/tls.key",
					Dir:      "/some/path/example.com",
				}

				paths, err := locator.Locate()

				Expect(err).NotTo(HaveOccurred())
				Expect(paths).To(ConsistOf(expectedPaths))
			})

		})

		Context("error handling", func() {
			Context("when the certificate does not include a host in its DNS names", func() {
				It("returns an error and an empty PemInfo", func() {
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: "-----BEGIN CERTIFICATE-----\nMIIC9DCCAdygAwIBAgIQONP6c751cFt4S7BfPak4hDANBgkqhkiG9w0BAQsFADAS\nMRAwDgYDVQQKEwdBY21lIENvMB4XDTE4MTExNDIyMjcwM1oXDTE5MTExNDIyMjcw\nM1owEjEQMA4GA1UEChMHQWNtZSBDbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC\nAQoCggEBALLY2/ZdkU7UZYi9Skbhm7SCmXpdMNRIUENKtfgu1aMl9nhH0kUFtzU8\nhwzIefJTqBJ7OQ8JlF6IN54PMWcJ8ZnXUpY30DY9A/LtEaYzjWasiaMi+XgU149r\niYUeH8PJlcsh1xQxtpdls0HqAbORoH6keZs0dW1JNkKJjWtdGBeTKpQxJOBjz8kp\n4pgrGLeV0OG2aQJUXbiSHzrYeOf7XvmIKrMbirM4ynt4IAM9TjFna5HTopcMCPYO\nzk1Huxr6n2xauDUIzMPAfBH7LVy809vGl52cLoUdQkcH4ijgapmpFR/305OcSHHh\n7v7Q1M7H7CtkzacskmqX7XciCGYNDxUCAwEAAaNGMEQwDgYDVR0PAQH/BAQDAgWg\nMBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwDwYDVR0RBAgwBocE\ntqgAATANBgkqhkiG9w0BAQsFAAOCAQEATz0PRREEX9MalXfEJoSwS+dPk2kphKpV\nSo4OoA3E+6MHHxYHE83/KMvaqq4ZoX2113ievL6y8pnevHXnWdhyZckzlyK5puxs\n50CSKw7PEXpJuqdBRa/ncnxcksPwwT2A7/WP8TVlv0zVqvwfqkOXsJsIiLxLzxeK\nxQ33XFOThRiC6oqNNOVBxV3d1QXGx64Q2tY0j9FHQNebJaMQyGy13tdzM4AQDB/t\nq0RnlEaFu2X4ecsrE9OcT0ru9qZ+jEA+14mBjxJcTfpE09DQ3xcxntjL60gVDmIP\n5zdVVLk1JtKBAUF0V0Omxikf6ZWl7arqMkNhb0BVi21roV673GKaaw==\n-----END CERTIFICATE-----\n",
						},
					}

					locator := certs.NewLocator("/some/path", pairs)
					paths, err := locator.Locate()

					Expect(err).To(MatchError("no DNS names provided in certificates"))
					Expect(paths).To(BeEmpty())
				})
			})

			Context("when passed invalid certs", func() {
				It("returns an error and an empty PemInfo", func() {
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: "gobbledygook",
						},
					}

					locator := certs.NewLocator("/some/path", pairs)
					paths, err := locator.Locate()

					Expect(err).To(MatchError("failed to decode cert pem"))
					Expect(paths).To(BeEmpty())
				})
			})

			Context("when the cert fails to parse", func() {
				It("returns an error and an empty PemInfo", func() {
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: "-----BEGIN PUBLIC KEY-----\nMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAlRuRnThUjU8/prwYxbty\nWPT9pURI3lbsKMiB6Fn/VHOKE13p4D8xgOCADpdRagdT6n4etr9atzDKUSvpMtR3\nCP5noNc97WiNCggBjVWhs7szEe8ugyqF23XwpHQ6uV1LKH50m92MbOWfCtjU9p/x\nqhNpQQ1AZhqNy5Gevap5k8XzRmjSldNAFZMY7Yv3Gi+nyCwGwpVtBUwhuLzgNFK/\nyDtw2WcWmUU7NuC8Q6MWvPebxVtCfVp/iQU6q60yyt6aGOBkhAX0LpKAEhKidixY\nnP9PNVBvxgu3XZ4P36gZV6+ummKdBVnc3NqwBLu5+CcdRdusmHPHd5pHf4/38Z3/\n6qU2a/fPvWzceVTEgZ47QjFMTCTmCwNt29cvi7zZeQzjtwQgn4ipN9NibRH/Ax/q\nTbIzHfrJ1xa2RteWSdFjwtxi9C20HUkjXSeI4YlzQMH0fPX6KCE7aVePTOnB69I/\na9/q96DiXZajwlpq3wFctrs1oXqBp5DVrCIj8hU2wNgB7LtQ1mCtsYz//heai0K9\nPhE4X6hiE0YmeAZjR0uHl8M/5aW9xCoJ72+12kKpWAa0SFRWLy6FejNYCYpkupVJ\nyecLk/4L1W0l6jQQZnWErXZYe0PNFcmwGXy1Rep83kfBRNKRy5tvocalLlwXLdUk\nAIU+2GKjyT3iMuzZxxFxPFMCAwEAAQ==\n-----END PUBLIC KEY-----",
						},
					}

					locator := certs.NewLocator("/some/path", pairs)
					paths, err := locator.Locate()

					Expect(err).To(HaveOccurred())
					Expect(paths).To(BeEmpty())
				})
			})

			Context("with cert chain provided with no DNSNames", func() {
				It("returns an error and an empty PemInfo", func() {
					certChain := createPEMSforCertChain([]string{})
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: fmt.Sprintf("%s%s", certChain.EndUserCert, certChain.RootCert),
						},
					}

					locator := certs.NewLocator("/some/path", pairs)
					paths, err := locator.Locate()

					Expect(err).To(MatchError("no DNS names provided in certificates"))
					Expect(paths).To(BeEmpty())
				})
			})
		})
	})

	var _ = Describe("Stow", func() {
		It("writes certs and keys to disk", func() {
			dnsNames := []string{"example.com"}
			certChain := createPEMSforCertChain(dnsNames)
			pairs := []certs.CertChainKeyPair{
				{
					CertChain:  certChain.EndUserCert,
					PrivateKey: certChain.EndUserPrivateKey,
				},
			}

			tempDir, err := ioutil.TempDir("", "certs")
			Expect(err).NotTo(HaveOccurred())

			locator := certs.NewLocator(tempDir, pairs)
			err = locator.Stow()
			Expect(err).NotTo(HaveOccurred())

			certFilePath := fmt.Sprintf("%s/example.com/tls.crt", tempDir)
			info, err := os.Stat(certFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
			cert, err := ioutil.ReadFile(certFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(cert)).To(Equal(certChain.EndUserCert))

			keyFilePath := fmt.Sprintf("%s/example.com/tls.key", tempDir)
			info, err = os.Stat(keyFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
			key, err := ioutil.ReadFile(keyFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(key)).To(Equal(certChain.EndUserPrivateKey))
		})

		Context("if a new certchain is provided in the config", func() {
			It("overwrites the files", func() {
				dnsNames := []string{"example.com"}
				certChain := createPEMSforCertChain(dnsNames)
				pairs := []certs.CertChainKeyPair{
					{
						CertChain:  certChain.EndUserCert,
						PrivateKey: certChain.EndUserPrivateKey,
					},
				}
				tempDir, err := ioutil.TempDir("", "certs")
				Expect(err).NotTo(HaveOccurred())

				err = os.Mkdir(fmt.Sprintf("%s/example.com", tempDir), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(fmt.Sprintf("%s/example.com/tls.crt", tempDir), []byte("blah blah"), 0600)
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(fmt.Sprintf("%s/example.com/tls.key", tempDir), []byte("blah blah"), 0600)
				Expect(err).NotTo(HaveOccurred())

				locator := certs.NewLocator(tempDir, pairs)
				err = locator.Stow()
				Expect(err).NotTo(HaveOccurred())

				certFilePath := fmt.Sprintf("%s/example.com/tls.crt", tempDir)
				Expect(err).NotTo(HaveOccurred())
				cert, err := ioutil.ReadFile(certFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(cert)).To(Equal(certChain.EndUserCert))

				keyFilePath := fmt.Sprintf("%s/example.com/tls.key", tempDir)
				Expect(err).NotTo(HaveOccurred())
				key, err := ioutil.ReadFile(keyFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(key)).To(Equal(certChain.EndUserPrivateKey))
			})
		})

		// error handling should be covered in Locate as the same private function
		// is used by both the Stow and Locate functions
	})
})

type certChainPEMs struct {
	EndUserCert       string
	EndUserPrivateKey string
	RootCert          string
}

func createPEMSforCertChain(dnsNames []string) certChainPEMs {
	rootPrivateKey, rootCADER := createCACertDER("theCA")
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	Expect(err).ToNot(HaveOccurred())

	subject := pkix.Name{Organization: []string{"xyz, Inc."}}

	certTemplate := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               subject,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 100),
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}

	rootCert, err := x509.ParseCertificate(rootCADER)
	Expect(err).NotTo(HaveOccurred())

	ownKey, err := rsa.GenerateKey(rand.Reader, 512)
	Expect(err).NotTo(HaveOccurred())

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, rootCert, &ownKey.PublicKey, rootPrivateKey)
	Expect(err).NotTo(HaveOccurred())

	ownKeyPEM, ownCertPEM := parsePEMfromDER(certDER, ownKey)
	_, rootCertPEM := parsePEMfromDER(rootCADER, rootPrivateKey)

	return certChainPEMs{
		EndUserCert:       string(ownCertPEM),
		EndUserPrivateKey: string(ownKeyPEM),
		RootCert:          string(rootCertPEM),
	}
}

func createCACertDER(cname string) (*rsa.PrivateKey, []byte) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	Expect(err).ToNot(HaveOccurred())

	subject := pkix.Name{Organization: []string{"xyz, Inc."}}
	if cname != "" {
		subject.CommonName = cname
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               subject,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 100),
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{cname},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 512)
	Expect(err).ToNot(HaveOccurred())
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &privKey.PublicKey, privKey)
	Expect(err).ToNot(HaveOccurred())
	return privKey, certDER
}

func parsePEMfromDER(certDER []byte, privKey *rsa.PrivateKey) (keyPEM, certPEM []byte) {
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	return
}
