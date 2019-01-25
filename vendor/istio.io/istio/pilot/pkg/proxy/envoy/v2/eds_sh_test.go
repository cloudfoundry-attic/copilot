// Copyright 2018 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package v2_test

import (
	"fmt"
	"testing"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	ads "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	proto "github.com/gogo/protobuf/types"

	meshconfig "istio.io/api/mesh/v1alpha1"
	testenv "istio.io/istio/mixer/test/client/env"
	"istio.io/istio/pilot/pkg/bootstrap"
	"istio.io/istio/pilot/pkg/model"
	v2 "istio.io/istio/pilot/pkg/proxy/envoy/v2"
	"istio.io/istio/pilot/pkg/serviceregistry"
	"istio.io/istio/pilot/pkg/serviceregistry/aggregate"
	"istio.io/istio/pkg/test/env"
	"istio.io/istio/tests/util"
)

// Testing the Split Horizon EDS.

type expectedResults struct {
	endpoints []string
	weights   map[string]uint32
}

// The test will setup 3 networks with various number of endpoints for the same service within
// each network. It creates an instance of memory registry for each cluster and populate it
// with Service, Instances and an ingress gateway service.
// It then conducts an EDS query from each network expecting results to match the design of
// the Split Horizon EDS - all local endpoints + endpoint per remote network that also has
// endpoints for the service.
func TestSplitHorizonEds(t *testing.T) {
	server, tearDown := initSplitHorizonTestEnv(t)
	defer tearDown()

	// Set up a cluster registry for network 1 with 1 instance for the service 'service5'
	// Network has 1 gateway
	initRegistry(server, 1, []string{"159.122.219.1"}, 1)
	// Set up a cluster registry for network 2 with 2 instances for the service 'service5'
	// Network has 1 gateway
	initRegistry(server, 2, []string{"159.122.219.2"}, 2)
	// Set up a cluster registry for network 3 with 3 instances for the service 'service5'
	// Network has 2 gateways
	initRegistry(server, 3, []string{"159.122.219.3", "179.114.119.3"}, 3)
	// Set up a cluster registry for network 4 with 4 instances for the service 'service5'
	// but without any gateway
	initRegistry(server, 4, []string{}, 4)

	tests := []struct {
		network   string
		sidecarID string
		want      expectedResults
	}{
		{
			// Verify that EDS from network1 will return 1 local endpoint with local VIP + 2 remote
			// endpoints weighted accordingly with the IP of the ingress gateway.
			network:   "network1",
			sidecarID: sidecarID("10.1.0.1", "app3"),
			want: expectedResults{
				endpoints: []string{"10.1.0.1", "159.122.219.2", "159.122.219.3", "179.114.119.3"},
				weights:   map[string]uint32{"159.122.219.2": 2, "159.122.219.3": 3},
			},
		},
		{
			// Verify that EDS from network2 will return 2 local endpoints with local VIPs + 2 remote
			// endpoints weighted accordingly with the IP of the ingress gateway.
			network:   "network2",
			sidecarID: sidecarID("10.2.0.1", "app3"),
			want: expectedResults{
				endpoints: []string{"10.2.0.1", "10.2.0.2", "159.122.219.1", "159.122.219.3", "179.114.119.3"},
				weights:   map[string]uint32{"159.122.219.1": 1, "159.122.219.3": 3, "179.114.119.3": 3},
			},
		},
		{
			// Verify that EDS from network3 will return 3 local endpoints with local VIPs + 2 remote
			// endpoints weighted accordingly with the IP of the ingress gateway.
			network:   "network3",
			sidecarID: sidecarID("10.3.0.1", "app3"),
			want: expectedResults{
				endpoints: []string{"10.3.0.1", "10.3.0.2", "10.3.0.3", "159.122.219.1", "159.122.219.2"},
				weights:   map[string]uint32{"159.122.219.1": 1, "159.122.219.2": 2},
			},
		},
		{
			// Verify that EDS from network4 will return 4 local endpoint with local VIP + 4 remote
			// endpoints weighted accordingly with the IP of the ingress gateway.
			network:   "network4",
			sidecarID: sidecarID("10.4.0.1", "app3"),
			want: expectedResults{
				endpoints: []string{"10.4.0.1", "10.4.0.2", "10.4.0.3", "10.4.0.4", "159.122.219.1", "159.122.219.2", "159.122.219.3", "179.114.119.3"},
				weights:   map[string]uint32{"159.122.219.1": 1, "159.122.219.2": 2, "159.122.219.3": 3, "179.114.119.3": 3},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.network, func(t *testing.T) {
			verifySplitHorizonResponse(t, tt.network, tt.sidecarID, tt.want)
		})
	}
}

// Tests whether an EDS response from the provided network matches the expected results
func verifySplitHorizonResponse(t *testing.T, network string, sidecarID string, expected expectedResults) {
	t.Helper()
	edsstr, cancel, err := connectADS(util.MockPilotGrpcAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	metadata := &proto.Struct{Fields: map[string]*proto.Value{
		"ISTIO_PROXY_VERSION": {Kind: &proto.Value_StringValue{StringValue: "1.1"}},
		"NETWORK":             {Kind: &proto.Value_StringValue{StringValue: network}},
	}}

	err = sendCDSReqWithMetadata(sidecarID, metadata, edsstr)
	if err != nil {
		t.Fatal(err)
	}
	_, err = adsReceive(edsstr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = sendEDSReqWithMetadata([]string{"outbound|1080||service5.default.svc.cluster.local"}, sidecarID, metadata, edsstr)
	if err != nil {
		t.Fatal(err)
	}
	res, err := adsReceive(edsstr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cla, err := getLoadAssignment(res)
	if err != nil {
		t.Fatal(err)
	}
	eps := cla.Endpoints

	if len(eps) != 1 {
		t.Fatal(fmt.Errorf("expecting 1 locality endpoint but got %d", len(eps)))
	}

	lbEndpoints := eps[0].LbEndpoints
	if len(lbEndpoints) != len(expected.endpoints) {
		t.Fatal(fmt.Errorf("number of endpoints should be %d but got %d", len(expected.endpoints), len(lbEndpoints)))
	}

	for addr, weight := range expected.weights {
		var match *endpoint.LbEndpoint
		for _, ep := range lbEndpoints {
			if ep.GetEndpoint().Address.GetSocketAddress().Address == addr {
				match = &ep
				break
			}
		}
		if match == nil {
			t.Fatal(fmt.Errorf("couldn't find endpoint with address %s", addr))
		}
		if match.LoadBalancingWeight.Value != weight {
			t.Fatal(fmt.Errorf("weight for endpoint %s is expected to be %d but got %d", addr, weight, match.LoadBalancingWeight.Value))
		}
	}
}

func initSplitHorizonTestEnv(t *testing.T) (*bootstrap.Server, util.TearDownFunc) {
	initMutex.Lock()
	defer initMutex.Unlock()
	testEnv = testenv.NewTestSetup(testenv.XDSTest, t)
	server, tearDown := util.EnsureTestServer()

	testEnv.Ports().PilotGrpcPort = uint16(util.MockPilotGrpcPort)
	testEnv.Ports().PilotHTTPPort = uint16(util.MockPilotHTTPPort)
	testEnv.IstioSrc = env.IstioSrc
	testEnv.IstioOut = env.IstioOut

	return server, tearDown
}

// initRegistry creates and initializes a memory registry that holds a single
// service with the provided amount of endpoints. It also creates a service for
// the ingress with the provided external IP
func initRegistry(server *bootstrap.Server, clusterNum int, gatewaysIP []string, numOfEndpoints int) {
	id := fmt.Sprintf("network%d", clusterNum)
	memRegistry := v2.NewMemServiceDiscovery(
		map[model.Hostname]*model.Service{}, 2)
	server.ServiceController.AddRegistry(aggregate.Registry{
		ClusterID:        id,
		Name:             serviceregistry.ServiceRegistry("memAdapter"),
		ServiceDiscovery: memRegistry,
		ServiceAccounts:  memRegistry,
		Controller:       &v2.MemServiceController{},
	})

	gws := []*meshconfig.Network_IstioNetworkGateway{}
	for _, gatewayIP := range gatewaysIP {
		if gatewayIP != "" {
			if server.EnvoyXdsServer.Env.MeshNetworks == nil {
				server.EnvoyXdsServer.Env.MeshNetworks = &meshconfig.MeshNetworks{
					Networks: map[string]*meshconfig.Network{},
				}
			}
			gw := &meshconfig.Network_IstioNetworkGateway{
				Gw: &meshconfig.Network_IstioNetworkGateway_Address{
					Address: gatewayIP,
				},
				Port: 80,
			}
			gws = append(gws, gw)
		}
	}

	if len(gws) != 0 {
		server.EnvoyXdsServer.Env.MeshNetworks.Networks[id] = &meshconfig.Network{
			Gateways: gws,
		}
	}

	svcLabels := map[string]string{
		"version": "v1.1",
	}

	// Explicit test service, in the v2 memory registry. Similar with mock.MakeService,
	// but easier to read.
	memRegistry.AddService("service5.default.svc.cluster.local", &model.Service{
		Hostname: "service5.default.svc.cluster.local",
		Address:  "10.10.0.1",
		Ports:    testPorts(0),
	})
	for i := 0; i < numOfEndpoints; i++ {
		memRegistry.AddInstance("service5.default.svc.cluster.local", &model.ServiceInstance{
			Endpoint: model.NetworkEndpoint{
				Address: fmt.Sprintf("10.%d.0.%d", clusterNum, i+1),
				Port:    2080,
				ServicePort: &model.Port{
					Name:     "http-main",
					Port:     1080,
					Protocol: model.ProtocolHTTP,
				},
				Network:  id,
				Locality: "az",
				UID:      "kubernetes://dummy",
			},
			Labels: svcLabels,
		})
	}
}

func sendCDSReqWithMetadata(node string, metadata *proto.Struct, edsstr ads.AggregatedDiscoveryService_StreamAggregatedResourcesClient) error {
	err := edsstr.Send(&xdsapi.DiscoveryRequest{
		ResponseNonce: time.Now().String(),
		Node: &core.Node{
			Id:       node,
			Metadata: metadata,
		},
		TypeUrl: v2.ClusterType})
	if err != nil {
		return fmt.Errorf("CDS request failed: %s", err)
	}

	return nil
}

func sendEDSReqWithMetadata(clusters []string, node string, metadata *proto.Struct,
	edsstr ads.AggregatedDiscoveryService_StreamAggregatedResourcesClient) error {
	err := edsstr.Send(&xdsapi.DiscoveryRequest{
		ResponseNonce: time.Now().String(),
		Node: &core.Node{
			Id:       node,
			Metadata: metadata,
		},
		TypeUrl:       v2.EndpointType,
		ResourceNames: clusters,
	})
	if err != nil {
		return fmt.Errorf("EDS request failed: %s", err)
	}

	return nil
}
