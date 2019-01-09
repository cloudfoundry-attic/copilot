package handlers_test

import (
	"context"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/handlers"
	"code.cloudfoundry.org/copilot/handlers/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VIP Resolver Handlers", func() {
	var (
		handler        *handlers.VIPResolver
		logger         lager.Logger
		fakeRoutesRepo *fakes.RoutesRepoVIPResolverInterface
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeRoutesRepo = &fakes.RoutesRepoVIPResolverInterface{}
		fakeRoutesRepo.GetVIPByNameReturns("1.2.3.4", true)

		handler = &handlers.VIPResolver{
			Logger:     logger,
			RoutesRepo: fakeRoutesRepo,
		}
	})

	Describe("Health", func() {
		It("always returns healthy", func() {
			ctx := context.Background()
			resp, err := handler.Health(ctx, new(api.HealthRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal(&api.HealthResponse{Healthy: true}))
		})
	})

	Describe("GetVIPByName", func() {
		It("returns a vip", func() {
			ctx := context.Background()
			resp, err := handler.GetVIPByName(ctx, &api.GetVIPByNameRequest{Fqdn: "meow.istio.apps.internal"})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Ip).To(Equal("1.2.3.4"))

			name := fakeRoutesRepo.GetVIPByNameArgsForCall(0)
			Expect(name).To(Equal("meow.istio.apps.internal"))
		})

		Context("when the route doesn't exist", func() {
			BeforeEach(func() {
				fakeRoutesRepo.GetVIPByNameReturns("", false)
			})

			It("returns an error", func() {
				ctx := context.Background()
				_, err := handler.GetVIPByName(ctx, &api.GetVIPByNameRequest{Fqdn: "meow.istio.apps.internal"})
				Expect(err).To(MatchError("route doesn't exist: meow.istio.apps.internal"))
			})
		})
	})
})
