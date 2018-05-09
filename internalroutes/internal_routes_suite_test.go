package internalroutes_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestInternalRoutes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InternalRoutes Suite")
}
