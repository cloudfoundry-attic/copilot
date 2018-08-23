package snapshot_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/snapshot"
	"code.cloudfoundry.org/copilot/snapshot/fakes"
	"code.cloudfoundry.org/lager/lagertest"

	networking "istio.io/api/networking/v1alpha3"
	snap "istio.io/istio/pkg/mcp/snapshot"

	"github.com/gogo/protobuf/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Run", func() {
	var (
		ticker    chan time.Time
		s         *snapshot.Snapshot
		collector *fakes.Collector
		setter    *fakes.Setter
	)

	BeforeEach(func() {
		l := lagertest.TestLogger{}
		ticker = make(chan time.Time)
		collector = &fakes.Collector{}
		setter = &fakes.Setter{}
		builder := snap.NewInMemoryBuilder()

		s = snapshot.New(l, ticker, collector, builder, setter)
	})

	It("sends mcp snapshots", func() {
		sig := make(chan os.Signal)
		ready := make(chan struct{})

		collector.CollectReturns([]*api.RouteWithBackends{
			{
				Hostname: "foo.example.com",
				Path:     "/something",
				Backends: &api.BackendSet{
					Backends: []*api.Backend{
						{
							Address: "10.10.10.1",
							Port:    uint32(65003),
						},
					},
				},
				CapiProcessGuid: "a-capi-guid",
				RouteWeight:     int32(100),
			},
		})

		go s.Run(sig, ready)
		ticker <- time.Time{}

		Eventually(setter.SetSnapshotCallCount).Should(Equal(1))
		node, shot := setter.SetSnapshotArgsForCall(0)
		Expect(node).To(Equal("copilot-node-id"))

		virtualServices := shot.Resources(snapshot.VSTypeURL)
		destinationRules := shot.Resources(snapshot.DRTypeURL)

		Expect(virtualServices).To(HaveLen(1))
		Expect(virtualServices[0].Metadata.Name).To(Equal("copilot-service-for-foo.example.com"))

		Expect(destinationRules).To(HaveLen(1))
		Expect(destinationRules[0].Metadata.Name).To(Equal("copilot-rule-for-foo.example.com"))

		var vs networking.VirtualService
		err := types.UnmarshalAny(virtualServices[0].Resource, &vs)
		Expect(err).NotTo(HaveOccurred())

		var dr networking.DestinationRule
		err = types.UnmarshalAny(destinationRules[0].Resource, &dr)
		Expect(err).NotTo(HaveOccurred())

		Expect(vs).To(Equal(networking.VirtualService{
			Hosts:    []string{"foo.example.com"},
			Gateways: []string{"cloudfoundry-ingress"},
			Http: []*networking.HTTPRoute{
				{
					Match: []*networking.HTTPMatchRequest{
						{
							Uri: &networking.StringMatch{
								MatchType: &networking.StringMatch_Prefix{
									Prefix: "/something",
								},
							},
						},
					},
					Route: []*networking.DestinationWeight{
						{
							Destination: &networking.Destination{
								Host: "foo.example.com",
								Port: &networking.PortSelector{
									Port: &networking.PortSelector_Number{
										Number: 8080,
									},
								},
								Subset: "a-capi-guid",
							},
							Weight: 100,
						},
					},
				},
			},
		}))
		Expect(dr).To(Equal(networking.DestinationRule{
			Host: "foo.example.com",
			Subsets: []*networking.Subset{
				{
					Name:   "a-capi-guid",
					Labels: map[string]string{"cfapp": "a-capi-guid"},
				},
			},
		}))

		sig <- os.Kill
	})
})
