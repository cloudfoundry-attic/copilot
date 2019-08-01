package db_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/cf-networking-helpers/db"
	"code.cloudfoundry.org/cf-networking-helpers/fakes"
	"github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	DATABASE_CLIENT_CERT = `-----BEGIN CERTIFICATE-----
MIIEJDCCAgygAwIBAgIRAPaxi331A4Tad6C4UzTuU80wDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEAxMHbXlzcWxDQTAeFw0xODEwMTYyMDQ0MjNaFw0yMDA0MTYyMDQz
MzdaMBYxFDASBgNVBAMTC215c3FsQ2xpZW50MIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAonUIEmTXRwXeE4VpCNwj0A92XGVXPBAUrIoxPPzQiy8ey9wR
JqWKCPQY/g2vkEme/4uNIN+o8iI4COYmPaIuRv1tqot4U2/mhfeDH79+E7oc97FX
AnksLTEni+zOBtJUOQd1KOF6TlyVP3PRn9m+QQxLU0qp5TPpSIuE11E4SeVShocx
65AcQPgmu5+YBKCSVN7J5hosXm8Wtd/d28lJ36WBumZFUa+qD3DMiWUqU+AcXeCh
GJOjiE5osalg9UaWSuMnag+wvTXEWYP3qd3eThFcfS4Bj9/1XABPvdlyug15behG
2iU1Ra4UO35zRb6jF6Ax9Tc+14FDzVsaHBNg1QIDAQABo3EwbzAOBgNVHQ8BAf8E
BAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQWBBRL
AKaQdBXjRQ6AsA7oduLGpBe7sTAfBgNVHSMEGDAWgBTVi+UqriPLeufRqMwe49lv
WU333DANBgkqhkiG9w0BAQsFAAOCAgEAaCUpDpTL2f5hQNh5eLeeEgdmnci+9ju7
MfLuhDbn9Ft4vyqoHUkbgyPThNBaA3ENWxu2Q4dsTshSAxg1QG50kZDTO9u5z/Ge
tuDGghzC6+Zw8xbfXUlpkSWgOcxUvVTKuPwNfoIgmjpFxmnJ96LeFT68ORspwrYo
7yh2ffn3oLIRMTKgVaoPqF3EzWcX/9Ij+TnnZQsfZkEQaMWrKorc/IioBLUsvhKt
YV4+GJb2jtr+T/kNgahMyeE6tZQWfAsvu11TA25QCyfuccy5EJcu89U5pqFR/0cf
jpvyUU/ODGqgMzOYEDkfZyrLfUUGCb4i1rv4uLNVIE/nzZdDM+Lw9tFL6oh8p9uP
Y8z2EqevhL2WN6RO+IpnpzMO3QZtzJMPCHFrLjHfAPD8qNB1HOp2mLQMe16ILd6P
rSFteZqLuBQ2rvej9JlU94+uMszV2/JVxmM2MZstyXGgqnD9TtLtWBKHAJg7gVqh
s+3Vg8CzDwJYMzVYRySkDIMBq+CTtxyt+AEinVum4PX8Zno43Z60k7VWQJZV1c3J
bi73lsmhEMhRmyC4rfIJQWF2r3ExV4Qhc8ISfUZgWJB7K3f1GxS1R5zGXa2GvOVQ
mar20iTXssU3qbyaOjVI3+8P1chBz3cUNLWT5WzZWB5EMdtHGyjCKnqE6XTZcVmG
Z8Xwl8mRjMI=
-----END CERTIFICATE-----`

	DATABASE_CLIENT_KEY = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAonUIEmTXRwXeE4VpCNwj0A92XGVXPBAUrIoxPPzQiy8ey9wR
JqWKCPQY/g2vkEme/4uNIN+o8iI4COYmPaIuRv1tqot4U2/mhfeDH79+E7oc97FX
AnksLTEni+zOBtJUOQd1KOF6TlyVP3PRn9m+QQxLU0qp5TPpSIuE11E4SeVShocx
65AcQPgmu5+YBKCSVN7J5hosXm8Wtd/d28lJ36WBumZFUa+qD3DMiWUqU+AcXeCh
GJOjiE5osalg9UaWSuMnag+wvTXEWYP3qd3eThFcfS4Bj9/1XABPvdlyug15behG
2iU1Ra4UO35zRb6jF6Ax9Tc+14FDzVsaHBNg1QIDAQABAoIBACnnuEpOWr2GRO+S
JTLU3iQIKQbSWTs0BrEvAF5z9DNC11XMkVv/rWh71oqJ6zRz2SCf1aqaJtE2hG+/
NjQFxpwnOQeZ7FLRdYwu+VLSKWpbQqedxgzsRrntiP7t+YMG9BS12MHPz6Ww+gqh
DHyIRSwwSKnWg5aM2msNGhoUaEmfBVzj7i2zaO2B3MGERMIC0ZYT2qwa4LOPyxOO
UkgXrsKaGPAlLLxR0q3uLhCueFpfQSwvdl7s03s9iz3gmdccwlpixhPtiBVOAwid
IfywswqYxcYj7jotv2LazuJQNPpfLiTDOiSGFPwzs1XJiZar2Q9aMjvfg6+s2Mzb
I7Tn0e0CgYEAwYM4smPnaz0jvJjuYd1hqPE+y+TmX4hQJfdt7nggmzQ2etKGiLbG
navrMu2KKnl+7PMQG/X3ok220ojybAZlOD89gvHQhRJsnI4a2BVCFB9u4MCnIfvJ
4CxTMH2qQ3Nux8r0CNuY6x2ZQqn7i7PPE+fJnkJJ2N5WTUaxFIsD4cMCgYEA1uqa
bSa/QsWbxivxy0/hzPtLin239b9SjgS8I/YJOVlbvb58Nz6SgOvcR/zbNf5GiiZu
p5iHSEvLchDZ2f9NSaUjnSExZ67fgIaUxdfVxzHFr64GHkFrJmzttnyLcwTVmKK0
xwQaLROsNvYoGUhoYLlGuG+Wlm3yLtlZKY/bMYcCgYBykxI3tSUo/nsxSE8kTKJt
F+F5cZ7hA2GJCTXikuejXUfAcvPK8IUqh8brUW+T9HmtK8Dm/TxQsbjEcOcwBJ1b
rz3pUOmIUL9T9mN4eyWzqmTI1+hdG6qMe1IKDO2JoEgALW9N609gLhc3PFO+hIjg
HUXn2RHGQOZSPL/ODP0QZwKBgAWnTkCo0Ec1Y4+nAElU5J+7zJTsEbbJPaa2wSxB
AKUdkKhBJotdfgUeL0FFiY62Daz8rdSC0qw4MjXh85kkeigBzBoKEX6kvwRmhete
biU7TfP9I/QPzH3KR8aRKCnyapwFS7Qgi3+8EL+xYgSoPvasaQvZA6EZa1GILixF
uIJpAoGBALXgHfbnc+BQDDfdmdGeRpD+3UyB0ThqTn7DDXjXTNlZ7tjcKjmMi1uw
oL2J2cUvbU3Y5R/ppJw0hQNgAXfiax1xuRATUHNweJXA57YwsbrRboxyyzPASQdk
NyGiWUJ68TPzhj9AkypCQfxhqK1Qve1TALPZ6N5lAE7erYsAJbGq
-----END RSA PRIVATE KEY-----`

	CERTIFICATE_FROM_ANOTHER_CA = `-----BEGIN CERTIFICATE-----
MIIEIzCCAgugAwIBAgIRAKRY04+EGcnAfPpeLf3dxZcwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEAxMHbXlzcWxDQTAeFw0xODEwMTYyMjU2NDdaFw0xODEwMTYyMzU2
NDdaMBUxEzARBgNVBAMTCmV4cGlyZVNvb24wggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCZ/8fZc0q1I03L78hro9jr987Tn6hsJNGod61GiWOHybHezt5i
+SOp7S/fdmLQsRSopUOSmlAH6ta5QbffGbtHY2NJQKJq7N8KRt6aSfbHxPDG96Rp
Q0OZZLyiEaFz2jECoTjqwZX9duG5wA1/AVnZEKqnbAWdIWP9AOTzwdJ/ne4CLzyj
Lm/HUNi9xsZvU5xgb8ZSW3z8SOf39UedocmDcA/rTZWAkO6ELPvx4KD6t5aBC4ir
k7tGveQFxTvziZr3lNZk+NTX2OWUrz5yoH/nMiXtHe4JuytFsN5DYF1f6/3Fxl19
AhkCkxTj238/FFLID34W7mfZbgN59ByBgnPxAgMBAAGjcTBvMA4GA1UdDwEB/wQE
AwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwHQYDVR0OBBYEFJuI
c/+CukBgDTwmU6A+6++WwzaKMB8GA1UdIwQYMBaAFFmJZ4pg0d2UuDKpQzF3XQAM
LandMA0GCSqGSIb3DQEBCwUAA4ICAQAh7+THwf0fe3syAaPVqnpx2kswUAqP9VTw
waxXswwp632JnQa9vctuVBQ7DNwOHSixaNlM7yR+w1FlubwLzNRR5EXOgi2kl5Le
mewKBmJLpMwkmAbpCUB2B2ofJJguMe0JVQC6OC3eA3JsTc1/FtqJ4H1+RD5xT6hx
uOxla3zwfynYD4WdRMAosYVJouCScgWJpK+MWEkMCx94GUcO4Ik9acWhzBcdgaUG
qjbtTq5dHgVwernhJaiuUC2R5wEvb3rkhav2TYHJucFm0NHFbMCCYNbFAp1t1OyW
hiNrGtUGN2jBoFZ9OEZaWuY00mKs0Elp5/ugHQ5hW6HXam/4Fh95PMBR1QC+c5AC
AhdCYEXpZXkjCe5vnXHegBxAMV2FU33G9rPWWAi76sBlqjApGaYfbYJW63bhEOZT
AtnHlrPVw/GM16KkzMEEbi4lRvY4F3F2FJ+LZSMKMNs9aX/CAAWs9up3n7PcePP0
fV70C2hVtCJbIfRPaWvrVAAktBP9xLTnzUvzijPLMEJ9o45vWdrtvyBFknQCpMts
lw6sWU26m2gvxs6CcX3yt0bt8SxjqyulqrOdFCVSjZbGMDaIamdEKnC6k5ySyizn
SM2qNm+nV5FhjsyMyzs6OuCNEZGDAqklWBAHHqLncb6elO9NZgDysB/xn6jS+zqT
F1Y5M6wvLA==
-----END CERTIFICATE-----`
)

var _ = Describe("MySQLConnectionStringBuilder", func() {
	Describe("Build", func() {
		var (
			mysqlConnectionStringBuilder *db.MySQLConnectionStringBuilder
			mySQLAdapter                 *fakes.MySQLAdapter
			config                       db.Config
		)

		BeforeEach(func() {
			config = db.Config{
				User:         "some-user",
				Password:     "some-password",
				Host:         "some-host",
				Port:         uint16(1234),
				DatabaseName: "some-database",
				Timeout:      5,
			}

			mySQLAdapter = &fakes.MySQLAdapter{}

			mysqlConnectionStringBuilder = &db.MySQLConnectionStringBuilder{
				MySQLAdapter: mySQLAdapter,
			}
			mySQLAdapter.ParseDSNStub = func(dsn string) (cfg *mysql.Config, err error) {
				return mysql.ParseDSN(dsn)
			}
		})

		It("builds a connection string", func() {
			connectionString, err := mysqlConnectionStringBuilder.Build(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(connectionString).To(Equal("some-user:some-password@tcp(some-host:1234)/some-database?parseTime=true&readTimeout=5s&timeout=5s&writeTimeout=5s"))
		})

		Context("when mysql.ParseDSN can't parse the connection string", func() {
			BeforeEach(func() {
				mySQLAdapter.ParseDSNReturns(nil, errors.New("foxtrot"))
			})

			It("returns an error", func() {
				_, err := mysqlConnectionStringBuilder.Build(config)
				Expect(err).To(MatchError("parsing db connection string: foxtrot"))
			})
		})

		Context("when requiring ssl", func() {
			var (
				caCertPool *x509.CertPool
			)

			BeforeEach(func() {
				caCertFile, err := ioutil.TempFile("", "")
				_, err = caCertFile.Write([]byte(DATABASE_CA_CERT))
				Expect(err).NotTo(HaveOccurred())

				config.RequireSSL = true
				config.CACert = caCertFile.Name()

				caCertPool = x509.NewCertPool()
				ok := caCertPool.AppendCertsFromPEM([]byte(DATABASE_CA_CERT))
				Expect(ok).To(BeTrue())
			})

			It("builds a tls connection string", func() {
				connectionString, err := mysqlConnectionStringBuilder.Build(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(connectionString).To(Equal("some-user:some-password@tcp(some-host:1234)/some-database?parseTime=true&readTimeout=5s&timeout=5s&tls=some-database-tls&writeTimeout=5s"))

				Expect(mySQLAdapter.RegisterTLSConfigCallCount()).To(Equal(1))
				passedTLSConfigName, passedTLSConfig := mySQLAdapter.RegisterTLSConfigArgsForCall(0)
				Expect(passedTLSConfigName).To(Equal("some-database-tls"))
				Expect(passedTLSConfig).To(Equal(&tls.Config{
					InsecureSkipVerify: false,
					RootCAs:            caCertPool,
				}))
			})

			Context("when SkipHostnameValidation is true", func() {
				BeforeEach(func() {
					config.SkipHostnameValidation = true
				})

				It("builds tls config skipping hostname", func() {
					connectionString, err := mysqlConnectionStringBuilder.Build(config)
					Expect(err).NotTo(HaveOccurred())
					Expect(connectionString).To(Equal("some-user:some-password@tcp(some-host:1234)/some-database?parseTime=true&readTimeout=5s&timeout=5s&tls=some-database-tls&writeTimeout=5s"))

					Expect(mySQLAdapter.RegisterTLSConfigCallCount()).To(Equal(1))
					passedTLSConfigName, passedTLSConfig := mySQLAdapter.RegisterTLSConfigArgsForCall(0)
					Expect(passedTLSConfigName).To(Equal("some-database-tls"))
					Expect(passedTLSConfig.InsecureSkipVerify).To(BeTrue())
					Expect(passedTLSConfig.RootCAs).To(Equal(caCertPool))
					Expect(passedTLSConfig.Certificates).To(BeNil())
					// impossible to assert VerifyPeerCertificate is set to a specfic function
					Expect(passedTLSConfig.VerifyPeerCertificate).NotTo(BeNil())
				})
			})

			Context("when it can't read the ca cert file", func() {
				BeforeEach(func() {
					config.CACert = "/foo/bar"
				})

				It("returns an error", func() {
					_, err := mysqlConnectionStringBuilder.Build(config)
					Expect(err).To(MatchError("reading db ca cert file: open /foo/bar: no such file or directory"))
				})
			})

			Context("when it can't append the ca cert to the cert pool", func() {
				BeforeEach(func() {
					caCertFile, err := ioutil.TempFile("", "")
					_, err = caCertFile.Write([]byte("bad cert"))
					Expect(err).NotTo(HaveOccurred())

					config.CACert = caCertFile.Name()
				})

				It("returns an error", func() {
					_, err := mysqlConnectionStringBuilder.Build(config)
					Expect(err).To(MatchError("appending cert to pool from pem - invalid cert bytes"))
				})
			})

			Context("when it can't register TLS config", func() {
				BeforeEach(func() {
					mySQLAdapter.RegisterTLSConfigReturns(errors.New("bad things happened"))
				})

				It("retruns an error", func() {
					_, err := mysqlConnectionStringBuilder.Build(config)
					Expect(err).To(MatchError("registering mysql tls config: bad things happened"))
				})
			})
		})
	})

	Describe("VerifyCertificatesIgnoreHostname", func() {
		var (
			caCertPool *x509.CertPool
		)

		BeforeEach(func() {
			caCertPool = x509.NewCertPool()
			ok := caCertPool.AppendCertsFromPEM([]byte(DATABASE_CA_CERT))
			Expect(ok).To(BeTrue())
		})

		It("verifies that provided certificates are valid", func() {
			block, _ := pem.Decode([]byte(DATABASE_CLIENT_CERT))

			err := db.VerifyCertificatesIgnoreHostname([][]byte{
				block.Bytes,
			}, caCertPool)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when raw certs are not parsable", func() {
			It("returns an error", func() {
				err := db.VerifyCertificatesIgnoreHostname([][]byte{
					[]byte("foo"),
					[]byte("bar"),
				}, nil)
				Expect(err.Error()).To(ContainSubstring("tls: failed to parse certificate from server: asn1: structure error: tags don't match"))
			})
		})

		Context("when verifying an expired cert", func() {
			It("returns an error", func() {
				block, _ := pem.Decode([]byte(CERTIFICATE_FROM_ANOTHER_CA))

				err := db.VerifyCertificatesIgnoreHostname([][]byte{
					block.Bytes,
				}, caCertPool)

				Expect(err.Error()).To(ContainSubstring("x509: certificate has expired or is not yet valid"))
			})
		})
	})
})
