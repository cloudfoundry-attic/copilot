package integration_test

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	ginkgoConfig "github.com/onsi/ginkgo/config"

	"testing"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var binaryPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	fmt.Fprintf(GinkgoWriter, "building binary...")
	binPath, err := gexec.Build("code.cloudfoundry.org/copilot", "-race")
	Expect(err).NotTo(HaveOccurred())
	fmt.Fprintf(GinkgoWriter, "done\n")
	return []byte(binPath)
}, func(data []byte) {
	binaryPath = string(data)
	rand.Seed(ginkgoConfig.GinkgoConfig.RandomSeed + int64(GinkgoParallelNode()))
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})
