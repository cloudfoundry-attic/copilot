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
				Path:     "",
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
			{
				Hostname: "foo.example.com",
				Path:     "/something",
				Backends: &api.BackendSet{
					Backends: []*api.Backend{
						{
							Address: "10.0.0.1",
							Port:    uint32(65005),
						},
					},
				},
				CapiProcessGuid: "x-capi-guid",
				RouteWeight:     int32(50),
			},
			{
				Hostname: "foo.example.com",
				Path:     "/something",
				Backends: &api.BackendSet{
					Backends: []*api.Backend{
						{
							Address: "10.0.0.0",
							Port:    uint32(65007),
						},
					},
				},
				CapiProcessGuid: "y-capi-guid",
				RouteWeight:     int32(50),
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
		serviceEntries := shot.Resources(snapshot.ServiceEntryTypeURL)

		Expect(virtualServices).To(HaveLen(1))
		Expect(virtualServices[0].Metadata.Name).To(Equal("copilot-service-for-foo.example.com"))

		Expect(destinationRules).To(HaveLen(1))
		Expect(destinationRules[0].Metadata.Name).To(Equal("copilot-rule-for-foo.example.com"))

		Expect(gateways).To(HaveLen(1))
		Expect(gateways[0].Metadata.Name).To(Equal("cloudfoundry-ingress"))

		Expect(serviceEntries).To(HaveLen(1))
		Expect(serviceEntries[0].Metadata.Name).To(Equal("copilot-service-entry-for-foo.example.com"))

		var vs networking.VirtualService
		err := types.UnmarshalAny(virtualServices[0].Resource, &vs)
		Expect(err).NotTo(HaveOccurred())

		var dr networking.DestinationRule
		err = types.UnmarshalAny(destinationRules[0].Resource, &dr)
		Expect(err).NotTo(HaveOccurred())

		var ga networking.Gateway
		err = types.UnmarshalAny(gateways[0].Resource, &ga)
		Expect(err).NotTo(HaveOccurred())

		var se networking.ServiceEntry
		err = types.UnmarshalAny(serviceEntries[0].Resource, &se)
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
								Subset: "x-capi-guid",
							},
							Weight: 50,
						},
						{
							Destination: &networking.Destination{
								Host: "foo.example.com",
								Port: &networking.PortSelector{
									Port: &networking.PortSelector_Number{
										Number: 8080,
									},
								},
								Subset: "y-capi-guid",
							},
							Weight: 50,
						},
					},
				},
				{
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
				{
					Name:   "x-capi-guid",
					Labels: map[string]string{"cfapp": "x-capi-guid"},
				},
				{
					Name:   "y-capi-guid",
					Labels: map[string]string{"cfapp": "y-capi-guid"},
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

		Expect(se).To(Equal(
			networking.ServiceEntry{
				Hosts: []string{"foo.example.com"},
				Ports: []*networking.Port{
					{
						Name:     "http",
						Number:   8080,
						Protocol: "http",
					},
				},
				Location:   networking.ServiceEntry_MESH_INTERNAL,
				Resolution: networking.ServiceEntry_STATIC, // do we need to think about DNS?
				Endpoints: []*networking.ServiceEntry_Endpoint{
					{
						Address: "10.10.10.1",
						Ports: map[string]uint32{
							"http": 65003,
						},
						Labels: map[string]string{"cfapp": "a-capi-guid"},
					},
					{
						Address: "10.0.0.1",
						Ports: map[string]uint32{
							"http": 65005,
						},
						Labels: map[string]string{"cfapp": "x-capi-guid"},
					},
					{
						Address: "10.0.0.0",
						Ports: map[string]uint32{
							"http": 65007,
						},
						Labels: map[string]string{"cfapp": "y-capi-guid"},
					},
				},
			}))

		sig <- os.Kill
	})
})
