package testhelpers

import (
	"net"
	"time"

	"github.com/square/certstrap/pkix"

	. "github.com/onsi/gomega"
)

type Credentials struct {
	CA, Key, Cert []byte
}

func GenerateCredentials(caCommonName string, commonName string) *Credentials {
	rootKey, err := pkix.CreateRSAKey(1024)
	Expect(err).NotTo(HaveOccurred())
	certAuthority, err := pkix.CreateCertificateAuthority(rootKey, "some-ou", time.Now().Add(1*time.Hour), "some-org", "some-country", "", "", caCommonName)
	Expect(err).NotTo(HaveOccurred())

	key, err := pkix.CreateRSAKey(1024)
	Expect(err).NotTo(HaveOccurred())
	csr, err := pkix.CreateCertificateSigningRequest(key, "some-ou", []net.IP{net.IPv4(127, 0, 0, 1)}, nil, "some-org", "some-country", "", "", commonName)
	Expect(err).NotTo(HaveOccurred())
	cert, err := pkix.CreateCertificateHost(certAuthority, rootKey, csr, time.Now().Add(1*time.Hour))
	Expect(err).NotTo(HaveOccurred())

	caBytes, err := certAuthority.Export()
	Expect(err).NotTo(HaveOccurred())
	keyBytes, err := key.ExportPrivate()
	Expect(err).NotTo(HaveOccurred())
	certBytes, err := cert.Export()
	Expect(err).NotTo(HaveOccurred())

	return &Credentials{CA: caBytes, Key: keyBytes, Cert: certBytes}
}
