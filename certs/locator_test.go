package certs_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/copilot/certs"
)

var _ = Describe("Locator", func() {
	var _ = Describe("Locate", func() {
		It("returns cert and key paths and their associated hostnames", func() {
			pairs := []certs.CertChainKeyPair{
				{
					CertChain: "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIRAPf9lECQDqNwfP1KpPxMqmIwDQYJKoZIhvcNAQELBQAw\nEjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xODExMTQxODU0MThaFw0xOTExMTQxODU0\nMThaMBIxEDAOBgNVBAoTB0FjbWUgQ28wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw\nggEKAoIBAQCgDRgg0xFS7Hw4yN/EMTVYp9+My7R6mZ/s7qYVEcrKVnLYZAAUsXsA\nLG1BVeTWfQSPvshi1EP4SAsRpZ8sO/o3GybVfm5ejBVOC0seA1zm2LHMwPyjeIXU\neM/7S3VdBkve+37vj78uZe149Jj+IkLL3zkfRtI+coG9mw4FpP0TqaRQ41cKqnQS\nD2iRbSfBW/nMRcFQr7aK+z+LQg6LPez7CxCsdXgcMf8kNVdceQSatEFnufnK/Gyy\nDs+P2ovlqLpVC05SsO/dTQp+QtVYMNeCA/eLixNzwfiCXhDZ993JFUWj3TkCr7f6\nBY5U/2naXAGS8ZZVzXlweX2SO0BYicPNAgMBAAGjTTBLMA4GA1UdDwEB/wQEAwIF\noDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMBYGA1UdEQQPMA2C\nC2V4YW1wbGUuY29tMA0GCSqGSIb3DQEBCwUAA4IBAQAbgykDDrDA00rKNx/B4G2j\nAeDAHkAnMK5IjdrgH2KUNeI07eRkLhYobrquwcKRYa9RJcM/eImX8BkviwjlOkDz\noJdU0LMVrsrjBuwj9qYg+D7IywvPrrrdrjgF05BxUfwoH1lTKm8Q9SVnpEWEdJj9\n+sP10reX+O7L4xiqgyuKHjWPEK4NJD+Wsw5n8UvIq5LVvTt3bLWsgN2Mole0lJb4\nvgR4N1absHZN6/yju5s7cY0lLBcEitJNUQeW3lHSOWXJ8xiw9aayFnJ4tmNWpALU\nvqettFbN38gfsH8JElHwyeKLthGL/Kj1Cvb//SbK30RnG8vY8kuqKKs3iwuHB+UU\n-----END CERTIFICATE-----",
				},
			}

			locator := certs.NewLocator(pairs)
			expectedThing := certs.CertPairPaths{
				Hosts:    []string{"example.com"},
				CertPath: "/etc/istio/example.com/tls.crt",
				KeyPath:  "/etc/istio/example.com/tls.key",
			}

			paths, err := locator.Locate()

			Expect(err).NotTo(HaveOccurred())
			Expect(paths).To(ConsistOf(expectedThing))
		})

		Context("when multiple hostnames and an IP are provided for a cert chain and key pair", func() {
			It("returns cert and key paths and their associated hostnames with the first hostname used in path", func() {
				pairs := []certs.CertChainKeyPair{
					{
						CertChain: "-----BEGIN CERTIFICATE-----\nMIIDHTCCAgWgAwIBAgIQCGefatAjgHQs9pOcWlv7ljANBgkqhkiG9w0BAQsFADAS\nMRAwDgYDVQQKEwdBY21lIENvMB4XDTE4MTExNDIyMTkyNloXDTE5MTExNDIyMTky\nNlowEjEQMA4GA1UEChMHQWNtZSBDbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC\nAQoCggEBALSt5r5z1u1FDvE+LlMYLNh5uNOIvUU4slEutDYYtqlC1IbB/n2VC8RR\nD1V73M0iDzQVr/F8Kt5mFk+9lvcGCbKY2KzjobYSWLgHF7dYJh5VlAjyMA9ZrtGR\nYS/An+SDFoQKmNl3tjjT/LcI0isrM+Z9qujKUAbImWF/zeTJjnyLBXbgW0RPzIbb\n59i1rSEQ7AVrpf5jvZl95dhdV35bJ11h6bIdEz2JdRyaTpfuKXP/zDmHV9ieEnZr\nqo113junw0fJw4EmJ55qovqMq1xzpyGwLpyoFRms3ivAVdS18KrYRxfGdVgEGB74\n4zBafddfXjvo6iSs8Hyj6zHcREy7DUcCAwEAAaNvMG0wDgYDVR0PAQH/BAQDAgWg\nMBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwOAYDVR0RBDEwL4IL\nZXhhbXBsZS5jb22CDGV4YW1wbGUyLmNvbYIMZXhhbXBsZTMuY29thwTAqAABMA0G\nCSqGSIb3DQEBCwUAA4IBAQBPhctmVjfCKc5H5ZEdjzDQQeCESfbvSAyYPuNYmaAX\nNmtlxiravYX5C8cgNZtBN22J8i3yRPNuclDxbCLXWrDZUFy2WGVNbbQK6whCgST4\nCrxETdcTZn3lvxa4J51VWZV5y47FRNBMOc7CU2hRgV2mXM69B8l8urlEOEom6dN3\nPFPnNNkBz28vA3DxZGa9JZgcECgq58H9hInsdgewlJyilb1yaRdMgLwZnZmR9nIz\nWK4B6BL3s4TzyJxPrYU/GXF/qHcsEvHom6m/KzNwkBpgUaSTq71QMzsk26ZhWn/h\ne5wy8LDuzQgOAHWS0ZSVLXWf1kfBgSHkYtzAbOGuWlaw\n-----END CERTIFICATE-----",
					},
				}

				locator := certs.NewLocator(pairs)
				expectedThing := certs.CertPairPaths{
					Hosts:    []string{"example.com", "example2.com", "example3.com"},
					CertPath: "/etc/istio/example.com/tls.crt",
					KeyPath:  "/etc/istio/example.com/tls.key",
				}

				paths, err := locator.Locate()

				Expect(err).NotTo(HaveOccurred())
				Expect(paths).To(ConsistOf(expectedThing))
			})
		})

		Context("When a certchain provided", func() {
			It("returns cert and key paths and their associated hostnames", func() {
				dnsNames := []string{"example.com", "example2.com"}
				certChain := createPEMSforCertChain(dnsNames)
				pairs := []certs.CertChainKeyPair{
					{
						CertChain: fmt.Sprintf("%s%s", certChain[0], certChain[1]),
					},
				}

				locator := certs.NewLocator(pairs)
				expectedThing := certs.CertPairPaths{
					Hosts:    dnsNames,
					CertPath: "/etc/istio/example.com/tls.crt",
					KeyPath:  "/etc/istio/example.com/tls.key",
				}

				paths, err := locator.Locate()

				Expect(err).NotTo(HaveOccurred())
				Expect(paths).To(ConsistOf(expectedThing))
			})

		})

		Context("error handling", func() {
			Context("when the certificate does not include a host in its DNS names", func() {
				It("returns an error and an empty CertPairPaths", func() {
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: "-----BEGIN CERTIFICATE-----\nMIIC9DCCAdygAwIBAgIQONP6c751cFt4S7BfPak4hDANBgkqhkiG9w0BAQsFADAS\nMRAwDgYDVQQKEwdBY21lIENvMB4XDTE4MTExNDIyMjcwM1oXDTE5MTExNDIyMjcw\nM1owEjEQMA4GA1UEChMHQWNtZSBDbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC\nAQoCggEBALLY2/ZdkU7UZYi9Skbhm7SCmXpdMNRIUENKtfgu1aMl9nhH0kUFtzU8\nhwzIefJTqBJ7OQ8JlF6IN54PMWcJ8ZnXUpY30DY9A/LtEaYzjWasiaMi+XgU149r\niYUeH8PJlcsh1xQxtpdls0HqAbORoH6keZs0dW1JNkKJjWtdGBeTKpQxJOBjz8kp\n4pgrGLeV0OG2aQJUXbiSHzrYeOf7XvmIKrMbirM4ynt4IAM9TjFna5HTopcMCPYO\nzk1Huxr6n2xauDUIzMPAfBH7LVy809vGl52cLoUdQkcH4ijgapmpFR/305OcSHHh\n7v7Q1M7H7CtkzacskmqX7XciCGYNDxUCAwEAAaNGMEQwDgYDVR0PAQH/BAQDAgWg\nMBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwDwYDVR0RBAgwBocE\ntqgAATANBgkqhkiG9w0BAQsFAAOCAQEATz0PRREEX9MalXfEJoSwS+dPk2kphKpV\nSo4OoA3E+6MHHxYHE83/KMvaqq4ZoX2113ievL6y8pnevHXnWdhyZckzlyK5puxs\n50CSKw7PEXpJuqdBRa/ncnxcksPwwT2A7/WP8TVlv0zVqvwfqkOXsJsIiLxLzxeK\nxQ33XFOThRiC6oqNNOVBxV3d1QXGx64Q2tY0j9FHQNebJaMQyGy13tdzM4AQDB/t\nq0RnlEaFu2X4ecsrE9OcT0ru9qZ+jEA+14mBjxJcTfpE09DQ3xcxntjL60gVDmIP\n5zdVVLk1JtKBAUF0V0Omxikf6ZWl7arqMkNhb0BVi21roV673GKaaw==\n-----END CERTIFICATE-----\n",
						},
					}

					locator := certs.NewLocator(pairs)
					paths, err := locator.Locate()

					Expect(err).To(MatchError("no DNS names provided in certificates"))
					Expect(paths).To(BeEmpty())
				})
			})

			Context("when passed invalid certs", func() {
				It("returns an error and an empty CertPairPaths", func() {
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: "gobbledygook",
						},
					}

					locator := certs.NewLocator(pairs)
					paths, err := locator.Locate()

					Expect(err).To(MatchError("failed to decode cert pem"))
					Expect(paths).To(BeEmpty())
				})
			})

			Context("when the cert fails to parse", func() {
				It("returns an error and an empty CertPairPaths", func() {
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: "-----BEGIN PUBLIC KEY-----\nMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAlRuRnThUjU8/prwYxbty\nWPT9pURI3lbsKMiB6Fn/VHOKE13p4D8xgOCADpdRagdT6n4etr9atzDKUSvpMtR3\nCP5noNc97WiNCggBjVWhs7szEe8ugyqF23XwpHQ6uV1LKH50m92MbOWfCtjU9p/x\nqhNpQQ1AZhqNy5Gevap5k8XzRmjSldNAFZMY7Yv3Gi+nyCwGwpVtBUwhuLzgNFK/\nyDtw2WcWmUU7NuC8Q6MWvPebxVtCfVp/iQU6q60yyt6aGOBkhAX0LpKAEhKidixY\nnP9PNVBvxgu3XZ4P36gZV6+ummKdBVnc3NqwBLu5+CcdRdusmHPHd5pHf4/38Z3/\n6qU2a/fPvWzceVTEgZ47QjFMTCTmCwNt29cvi7zZeQzjtwQgn4ipN9NibRH/Ax/q\nTbIzHfrJ1xa2RteWSdFjwtxi9C20HUkjXSeI4YlzQMH0fPX6KCE7aVePTOnB69I/\na9/q96DiXZajwlpq3wFctrs1oXqBp5DVrCIj8hU2wNgB7LtQ1mCtsYz//heai0K9\nPhE4X6hiE0YmeAZjR0uHl8M/5aW9xCoJ72+12kKpWAa0SFRWLy6FejNYCYpkupVJ\nyecLk/4L1W0l6jQQZnWErXZYe0PNFcmwGXy1Rep83kfBRNKRy5tvocalLlwXLdUk\nAIU+2GKjyT3iMuzZxxFxPFMCAwEAAQ==\n-----END PUBLIC KEY-----",
						},
					}

					locator := certs.NewLocator(pairs)
					paths, err := locator.Locate()

					Expect(err).To(HaveOccurred())
					Expect(paths).To(BeEmpty())
				})
			})

			Context("with cert chain provided with no DNSNames", func() {
				It("returns an error and an empty CertPairPaths", func() {
					certChain := createPEMSforCertChain([]string{})
					pairs := []certs.CertChainKeyPair{
						{
							CertChain: fmt.Sprintf("%s%s", certChain[0], certChain[1]),
						},
					}

					locator := certs.NewLocator(pairs)
					paths, err := locator.Locate()

					Expect(err).To(MatchError("no DNS names provided in certificates"))
					Expect(paths).To(BeEmpty())
				})
			})
		})
	})
})

func createPEMSforCertChain(dnsNames []string) []string {
	rootPrivateKey, rootCADER := CreateCACertDER("theCA")
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

	_, ownCertPEM := parsePEMfromDER(certDER, ownKey)
	_, rootCertPEM := parsePEMfromDER(rootCADER, rootPrivateKey)

	return []string{string(ownCertPEM), string(rootCertPEM)}
}

func CreateCACertDER(cname string) (*rsa.PrivateKey, []byte) {
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
