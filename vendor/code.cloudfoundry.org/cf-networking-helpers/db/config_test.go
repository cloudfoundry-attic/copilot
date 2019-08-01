package db_test

import (
	"io/ioutil"

	"code.cloudfoundry.org/cf-networking-helpers/db"

	"github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	DATABASE_CA_CERT = `-----BEGIN CERTIFICATE-----
MIIE5DCCAsygAwIBAgIBATANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDEwdteXNx
bENBMB4XDTE4MTAxNjIwNDMzN1oXDTIwMDQxNjIwNDMzN1owEjEQMA4GA1UEAxMH
bXlzcWxDQTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBANiduEyqVkST
hwG7TnihP/GHpvzJwdJVuhxAa0Qd++uo8VsD8cqHKX6IbmwcVUgrBDikY0kaks7g
el5bwUDg4oi/1pTdT+vbrOZLCgksO043/34zLe2itVsYVABnvqMnOZM+bAr7Im9V
wb6rfXAGulow1vmzuzxGtGGV4x5ETCOA+SnytxHH1L57RgAmUJnEK/Ks4CEWoByR
v8kejd8KScAo2pZ2ldlXJk+ggSifbyxCrMaA/E0HfA2epnhBy9nhbBXx4/p35SIW
B+Nzv9FMmb8PZ0AHw7PEg3WJzrrrYZXyKZL1EmGT9w0FzxcirYFmmWL/Zzm4lkHM
YvwOI3eCPoPFYJcgzJuJbtbcXkiCIZ2M5QunIGKgACBDw/YHNrZMHGCFplZxT2E0
rJTq01ZWC4wKhpKGtTkvZzR9SC3tyK/hbcjjP+jtSc3ZA7V2h8Lwa8ZfydurJQ2n
fgKpiaXpiqvv36wpfXzieywooioh86qsZhUs39V8JPr8HK11PXKT9ZRwmdcTjcBA
9gfauxUBa/NKAGIPoxe0+1JTWFwxt85hEZPMq/A+6zPH6smgDVV8SayEoYQQ7yLr
/mtKmEZ7QTl/0prTBQPsfIowvHr2MIPOiNrbuoUvV+gKZfhGMeLmOa8BxQ/FHiUe
WZ5Xhd9a0xnOnQb+QdQAfAoEaMYdPeQpAgMBAAGjRTBDMA4GA1UdDwEB/wQEAwIB
BjASBgNVHRMBAf8ECDAGAQH/AgEAMB0GA1UdDgQWBBTVi+UqriPLeufRqMwe49lv
WU333DANBgkqhkiG9w0BAQsFAAOCAgEAUacizdIDaONqXYaERZky5vsTS8rCrL6w
bHj9n9iGpx9irMREnEUFuecttkuZbDWQqXiH//vuUbbE4YoiVwT2GlLlBAvpYsXY
qDn011c7ZjU4Aw5voJpaR25lCpVrztev5//Cyz9KIm0aZ01ARjWDcPSg4GRyjnFj
fIszidqquRr0lrutNEBjyKxibMoDzkgbXXCdh3hjdVJX7zhMN7wcFqcx5/8f+fb3
EcO0qiT385LUh3qlhg5w9t/gxblwsnQK6X241O8nDoZgxvnW62RN4GqUn1ZtLeGs
pylBCQ/CePSasDN4mJRmPxMKKiKJyp0XdvgSagseq5kW+Zaz6H04QdCjgBOgrPdD
UkWnb8hQiVsboPxpc//a0JIsXZ0krb2UkSv4JrIYOVa/4lRaj9Ie5weBZkYmd+Kp
7f//4UezWSDfWv7S1GbxF+d8rWkcZksV9/es2GhspH6oM2GtE+7198R12XIq8aVk
X7e056LpGxjMy6rvnVId3NwITdk6VB5SnsJL0RaGIu1YXhs9HR/Q9TCuWABJEJ/M
P1zzwuCu9cIOfXYzAGV5miS6FsgQEvxFNp15U4bS/Mbrct/6Z6JFo96ueSjOvehb
RSper1U+5n6G+LEHYrn8mpl1T/YkVTgmrKrxNFdsw9YYWFhut8Mh/N04Pd5Y1yzi
hB1P/1ZlKVU=
-----END CERTIFICATE-----`
)

var _ = Describe("Config", func() {
	var (
		config db.Config
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
	})

	Describe("ConnectionString", func() {
		Context("when the type is postgres", func() {
			BeforeEach(func() {
				config.Type = "postgres"
			})

			It("returns the connection string", func() {
				connectionString, err := config.ConnectionString()
				Expect(err).NotTo(HaveOccurred())
				Expect(connectionString).To(Equal("postgres://some-user:some-password@some-host:1234/some-database?sslmode=disable&connect_timeout=5000"))
			})
		})

		Context("when the type is mysql", func() {
			BeforeEach(func() {
				config.Type = "mysql"
			})

			It("returns the connection string", func() {
				connectionString, err := config.ConnectionString()
				Expect(err).NotTo(HaveOccurred())
				Expect(connectionString).To(Equal("some-user:some-password@tcp(some-host:1234)/some-database?parseTime=true&readTimeout=5s&timeout=5s&writeTimeout=5s"))
			})

			Context("when require_ssl is enabled", func() {
				BeforeEach(func() {
					config.RequireSSL = true
				})

				AfterEach(func() {
					mysql.DeregisterTLSConfig("some-database-tls")
				})

				Context("success", func() {
					BeforeEach(func() {
						caCertFile, err := ioutil.TempFile("", "")
						Expect(err).NotTo(HaveOccurred())

						_, err = caCertFile.Write([]byte(DATABASE_CA_CERT))
						Expect(err).NotTo(HaveOccurred())

						config.CACert = caCertFile.Name()
					})

					It("returns the amended connection string", func() {
						connectionString, err := config.ConnectionString()
						Expect(err).NotTo(HaveOccurred())
						Expect(connectionString).To(Equal("some-user:some-password@tcp(some-host:1234)/some-database?parseTime=true&readTimeout=5s&timeout=5s&tls=some-database-tls&writeTimeout=5s"))
					})
				})

				Context("when reading the cert file fails", func() {
					BeforeEach(func() {
						config.CACert = "garbage"
					})

					It("returns an error", func() {
						_, err := config.ConnectionString()
						Expect(err).To(HaveOccurred())
						Expect(err).To(MatchError("reading db ca cert file: open garbage: no such file or directory"))
					})
				})

				Context("when adding the cert to the pool fails", func() {
					BeforeEach(func() {
						caCertFile, err := ioutil.TempFile("", "")
						Expect(err).NotTo(HaveOccurred())

						_, err = caCertFile.Write([]byte("garbage"))
						Expect(err).NotTo(HaveOccurred())

						config.CACert = caCertFile.Name()
					})

					It("returns an error", func() {
						_, err := config.ConnectionString()
						Expect(err).To(HaveOccurred())
						Expect(err).To(MatchError("appending cert to pool from pem - invalid cert bytes"))
					})
				})
			})
		})

		Context("when the type is neither", func() {
			BeforeEach(func() {
				config.Type = "neither"
			})

			It("returns an error", func() {
				_, err := config.ConnectionString()
				Expect(err).To(MatchError("database type 'neither' is not supported"))
			})
		})

		Context("when the timeout is less than 1", func() {
			BeforeEach(func() {
				config.Type = "postgres"
				config.Timeout = 0
			})

			It("returns an error", func() {
				_, err := config.ConnectionString()
				Expect(err).To(MatchError("timeout must be at least 1 second: 0"))
			})
		})
	})
})
