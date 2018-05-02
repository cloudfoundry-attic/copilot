package vip_test

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/copilot/vip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Provider", func() {

	var provider *vip.Provider
	BeforeEach(func() {
		provider = &vip.Provider{}
	})

	It("returns a parsable IP", func() {
		Expect(net.ParseIP(provider.Get("a-hostname.apps.internal"))).ToNot(BeNil())
	})

	Specify("the same hostname always returns the same VIP", func() {
		vip1 := provider.Get("potato")
		vip2 := provider.Get("potato")
		vip3 := provider.Get("potato")

		Expect(vip1).To(Equal(vip2))
		Expect(vip1).To(Equal(vip3))
		Expect(vip2).To(Equal(vip3))
	})

	Specify("different hostnames return different vips", func() {
		vip1 := provider.Get("potato")
		vip2 := provider.Get("banana")
		vip3 := provider.Get("fruitcake")

		Expect(vip1).NotTo(Equal(vip2))
		Expect(vip1).NotTo(Equal(vip3))
		Expect(vip2).NotTo(Equal(vip3))
	})

	It("never returns 127.0.0.1 or 127.0.0.0", func() {
		for i := 0; i < 10000; i++ {
			vip := provider.Get(fmt.Sprintf("%d", i))
			Expect(vip).NotTo(Equal("127.0.0.1"), fmt.Sprintf("failed on %d", i))
			Expect(vip).NotTo(Equal("127.0.0.0"), fmt.Sprintf("failed on %d", i))
		}
	})

	It("uses the full range of 127/8", func() {
		foundPrefixes := map[byte]interface{}{}
		for i := 0; i < 10000; i++ {
			vipStr := provider.Get(fmt.Sprintf("%d", i))
			vip := net.ParseIP(vipStr)
			foundPrefixes[vip.To4()[1]] = true
		}
		Expect(foundPrefixes).To(HaveLen(256))
	})
})
