package snapshot_test

import (
	"code.cloudfoundry.org/policy_client"
	"errors"
	"github.com/onsi/gomega/gbytes"
	"os"
	"time"

	"code.cloudfoundry.org/copilot/snapshot"
	"code.cloudfoundry.org/copilot/snapshot/fakes"
	"code.cloudfoundry.org/lager/lagertest"
	policyClientFake "code.cloudfoundry.org/policy_client/fakes"

	snap "istio.io/istio/pkg/mcp/snapshot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Snapshot", func() {
	var _ = Describe("Run", func() {
		var (
			ticker             chan time.Time
			s                  *snapshot.Snapshot
			collector          *fakes.Collector
			setter             *fakes.Setter
			builder            *snap.InMemoryBuilder
			config             *fakes.Config
			policyServerClient *policyClientFake.InternalPolicyClient
			fakePolicies       []*policy_client.Policy
			logger             *lagertest.TestLogger
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("snapshot")
			ticker = make(chan time.Time)
			collector = &fakes.Collector{}
			setter = &fakes.Setter{}
			builder = snap.NewInMemoryBuilder()
			config = &fakes.Config{}
			policyServerClient = &policyClientFake.InternalPolicyClient{}
			fakePolicies = []*policy_client.Policy{}

			s = snapshot.New(logger, ticker, collector, setter, builder, policyServerClient, config)
		})

		It("does nothing if there are no changes", func() {
			sig := make(chan os.Signal)
			ready := make(chan struct{})

			collector.CollectReturnsOnCall(0, routesWithBackends())
			collector.CollectReturnsOnCall(1, routesWithBackends())
			policyServerClient.GetPoliciesReturns(fakePolicies, nil, nil)

			go s.Run(sig, ready)
			ticker <- time.Time{}

			Eventually(config.CreateGatewayResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateVirtualServiceResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateDestinationRuleResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateServiceEntryResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateSidecarResourcesCallCount).Should(Equal(1))

			Eventually(setter.SetSnapshotCallCount).Should(Equal(1))
			node, shot := setter.SetSnapshotArgsForCall(0)
			Expect(node).To(Equal("default"))
			checkVersion(shot, "1")

			ticker <- time.Time{}

			Eventually(config.CreateGatewayResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateVirtualServiceResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateDestinationRuleResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateServiceEntryResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateSidecarResourcesCallCount).Should(Equal(1))

			Consistently(setter.SetSnapshotCallCount).Should(Equal(1))

			sig <- os.Kill
		})

		It("initializes the builder a single time even when empty", func() {
			sig := make(chan os.Signal)
			ready := make(chan struct{})

			collector.CollectReturnsOnCall(0, nil)

			go s.Run(sig, ready)
			ticker <- time.Time{}

			Eventually(setter.SetSnapshotCallCount).Should(Equal(1))

			sig <- os.Kill
		})

		It("creates resources and sets the snapshot with the correct versions when routes change", func() {
			sig := make(chan os.Signal)
			ready := make(chan struct{})

			collector.CollectReturnsOnCall(0, routesWithBackends())
			collector.CollectReturnsOnCall(1, routesWithBackends()[1:])
			policyServerClient.GetPoliciesReturns(fakePolicies, nil, nil)

			go s.Run(sig, ready)
			ticker <- time.Time{}

			Eventually(config.CreateGatewayResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateVirtualServiceResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateDestinationRuleResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateServiceEntryResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateSidecarResourcesCallCount).Should(Equal(1))

			Eventually(setter.SetSnapshotCallCount).Should(Equal(1))
			node, shot := setter.SetSnapshotArgsForCall(0)
			Expect(node).To(Equal("default"))
			checkVersion(shot, "1")

			ticker <- time.Time{}

			Eventually(config.CreateGatewayResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateVirtualServiceResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateDestinationRuleResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateServiceEntryResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateSidecarResourcesCallCount).Should(Equal(2))

			Eventually(setter.SetSnapshotCallCount).Should(Equal(2))
			_, shot = setter.SetSnapshotArgsForCall(1)
			checkVersion(shot, "2")

			sig <- os.Kill
		})

		It("creates resources and sets the snapshot with the correct versions when policies change", func() {
			sig := make(chan os.Signal)
			ready := make(chan struct{})

			collector.CollectReturns(routesWithBackends())
			policyServerClient.GetPoliciesReturnsOnCall(0, fakePolicies, nil, nil)
			changedPolicies := []*policy_client.Policy{{}}
			policyServerClient.GetPoliciesReturnsOnCall(1, changedPolicies, nil, nil)

			go s.Run(sig, ready)
			ticker <- time.Time{}

			Eventually(config.CreateGatewayResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateVirtualServiceResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateDestinationRuleResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateServiceEntryResourcesCallCount).Should(Equal(1))
			Eventually(config.CreateSidecarResourcesCallCount).Should(Equal(1))
			_, policies, _ := config.CreateSidecarResourcesArgsForCall(0)
			Expect(policies).To(Equal(fakePolicies))

			Eventually(setter.SetSnapshotCallCount).Should(Equal(1))
			node, shot := setter.SetSnapshotArgsForCall(0)
			Expect(node).To(Equal("default"))
			checkVersion(shot, "1")

			ticker <- time.Time{}

			Eventually(config.CreateGatewayResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateVirtualServiceResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateDestinationRuleResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateServiceEntryResourcesCallCount).Should(Equal(2))
			Eventually(config.CreateSidecarResourcesCallCount).Should(Equal(2))

			Eventually(setter.SetSnapshotCallCount).Should(Equal(2))
			_, shot = setter.SetSnapshotArgsForCall(1)
			checkVersion(shot, "2")

			sig <- os.Kill
		})

		It("exits without an error", func() {
			sig := make(chan os.Signal)
			ready := make(chan struct{})
			errCh := make(chan error)
			go func() {
				errCh <- s.Run(sig, ready)
			}()

			sig <- os.Kill

			var err error
			Eventually(errCh).Should(Receive(&err))
			Expect(err).To(BeNil())
		})

		It("logs an error coming from the policy client and does not generate a snapshot", func() {
			sig := make(chan os.Signal)
			ready := make(chan struct{})

			collector.CollectReturns(routesWithBackends())
			policyServerClient.GetPoliciesReturns(nil, nil, errors.New("Policy Error"))

			go s.Run(sig, ready)
			ticker <- time.Time{}

			Eventually(logger).Should(gbytes.Say("Policy Error"))
			Expect(setter.SetSnapshotCallCount()).To(Equal(0))

			sig <- os.Kill
		})
	})
})

func checkVersion(shot snap.Snapshot, version string) {
	vsVersion := shot.Version(snapshot.VirtualServiceTypeURL)
	Expect(vsVersion).To(Equal(version))

	drVersion := shot.Version(snapshot.DestinationRuleTypeURL)
	Expect(drVersion).To(Equal(version))

	// Gateway version is always 1
	gaVersion := shot.Version(snapshot.GatewayTypeURL)
	Expect(gaVersion).To(Equal("1"))

	seVersion := shot.Version(snapshot.ServiceEntryTypeURL)
	Expect(seVersion).To(Equal(version))

	scVersion := shot.Version(snapshot.SidecarTypeURL)
	Expect(scVersion).To(Equal("1"))
}
