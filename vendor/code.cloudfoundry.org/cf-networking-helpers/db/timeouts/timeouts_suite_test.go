package timeouts_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTimeouts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Timeouts Suite")
}
