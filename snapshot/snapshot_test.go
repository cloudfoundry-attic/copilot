package snapshot_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/copilot/snapshot"
	"code.cloudfoundry.org/copilot/snapshot/fakes"
	"code.cloudfoundry.org/lager/lagertest"

	networking "istio.io/api/networking/v1alpha3"

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

		s = snapshot.New(l, ticker, collector, setter)
	})

	It("mcp snapshots sends gateways, virutalServices and destinationRules", func() {
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
		Expect(node).To(Equal(""))

		virtualServices := shot.Resources(snapshot.VirtualServiceTypeURL)
		destinationRules := shot.Resources(snapshot.DestinationRuleTypeURL)
		gateways := shot.Resources(snapshot.GatewayTypeURL)

		Expect(virtualServices).To(HaveLen(1))
		Expect(virtualServices[0].Metadata.Name).To(Equal("copilot-service-for-foo.example.com"))

		Expect(destinationRules).To(HaveLen(1))
		Expect(destinationRules[0].Metadata.Name).To(Equal("copilot-rule-for-foo.example.com"))

		Expect(gateways).To(HaveLen(1))
		Expect(gateways[0].Metadata.Name).To(Equal("cloudfoundry-ingress"))

		var vs networking.VirtualService
		err := types.UnmarshalAny(virtualServices[0].Resource, &vs)
		Expect(err).NotTo(HaveOccurred())

		var dr networking.DestinationRule
		err = types.UnmarshalAny(destinationRules[0].Resource, &dr)
		Expect(err).NotTo(HaveOccurred())

		var ga networking.Gateway
		err = types.UnmarshalAny(gateways[0].Resource, &ga)
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

		Expect(ga).To(Equal(
			networking.Gateway{
				Servers: []*networking.Server{
					&networking.Server{
						Port: &networking.Port{
							Number:   80,
							Protocol: "http",
							Name:     "http",
						},
						Hosts: []string{"*"},
					},
				},
			}))

		sig <- os.Kill
	})
})
