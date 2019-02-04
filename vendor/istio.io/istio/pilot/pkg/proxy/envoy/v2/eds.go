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

package v2

import (
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/types"
	"github.com/prometheus/client_golang/prometheus"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/util"
	"istio.io/istio/pilot/pkg/serviceregistry/aggregate"
)

// EDS returns the list of endpoints (IP:port and in future labels) associated with a real
// service or a subset of a service, selected using labels.
//
// The source of info is a list of service registries.
//
// Primary event is an endpoint creation/deletion. Once the event is fired, EDS needs to
// find the list of services associated with the endpoint.
//
// In case of k8s, Endpoints event is fired when the endpoints are added to service - typically
// after readiness check. At that point we have the 'real' Service. The Endpoint includes a list
// of port numbers and names.
//
// For the subset case, the Pod referenced in the Endpoint must be looked up, and pod checked
// for labels.
//
// In addition, ExternalEndpoint includes IPs and labels directly and can be directly processed.
//
// TODO: for selector-less services (mesh expansion), skip pod processing
// TODO: optimize the code path for ExternalEndpoint, no additional processing needed
// TODO: if a service doesn't have split traffic - we can also skip pod and lable processing
// TODO: efficient label processing. In alpha3, the destination policies are set per service, so
// we may only need to search in a small list.

var (
	edsClusterMutex sync.RWMutex
	edsClusters     = map[string]*EdsCluster{}

	// Tracks connections, increment on each new connection.
	connectionNumber = int64(0)
)

// EdsCluster tracks eds-related info for monitored clusters. In practice it'll include
// all clusters until we support on-demand cluster loading.
type EdsCluster struct {
	// mutex protects changes to this cluster
	mutex sync.Mutex

	LoadAssignment *xdsapi.ClusterLoadAssignment

	// FirstUse is the time the cluster was first used, for debugging
	FirstUse time.Time

	// EdsClients keeps track of all nodes monitoring the cluster.
	EdsClients map[string]*XdsConnection `json:"-"`

	// NonEmptyTime is the time the cluster first had a non-empty set of endpoints
	NonEmptyTime time.Time

	// The discovery service this cluster is associated with.
	discovery *DiscoveryServer
}

// TODO: add prom metrics !

// Return the load assignment. The field can be updated by another routine.
func loadAssignment(c *EdsCluster) *xdsapi.ClusterLoadAssignment {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.LoadAssignment
}

func buildEnvoyLbEndpoint(UID string, family model.AddressFamily, address string, port uint32, network string) *endpoint.LbEndpoint {
	var addr core.Address
	switch family {
	case model.AddressFamilyTCP:
		addr = core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: address,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: port,
					},
				},
			},
		}
	case model.AddressFamilyUnix:
		addr = core.Address{Address: &core.Address_Pipe{Pipe: &core.Pipe{Path: address}}}
	}

	ep := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: &addr,
			},
		},
	}

	// Istio telemetry depends on the metadata value being set for endpoints in the mesh.
	// Do not remove: mixerfilter depends on this logic.
	ep.Metadata = endpointMetadata(UID, network)

	//log.Infoa("EDS: endpoint ", ipAddr, ep.String())
	return ep
}

func networkEndpointToEnvoyEndpoint(e *model.NetworkEndpoint) (*endpoint.LbEndpoint, error) {
	err := model.ValidateNetworkEndpointAddress(e)
	if err != nil {
		return nil, err
	}
	addr := util.GetNetworkEndpointAddress(e)
	ep := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: &addr,
			},
		},
	}

	// Istio telemetry depends on the metadata value being set for endpoints in the mesh.
	// Do not remove: mixerfilter depends on this logic.
	ep.Metadata = endpointMetadata(e.UID, e.Network)

	//log.Infoa("EDS: endpoint ", ipAddr, ep.String())
	return ep, nil
}

// Create an Istio filter metadata object with the UID and Network fields (if exist).
func endpointMetadata(UID string, network string) *core.Metadata {
	if UID == "" && network == "" {
		return nil
	}

	metadata := &core.Metadata{
		FilterMetadata: map[string]*types.Struct{
			util.IstioMetadataKey: {
				Fields: map[string]*types.Value{},
			},
		},
	}

	if UID != "" {
		metadata.FilterMetadata["istio"].Fields["uid"] = &types.Value{Kind: &types.Value_StringValue{StringValue: UID}}
	}

	if network != "" {
		metadata.FilterMetadata["istio"].Fields["network"] = &types.Value{Kind: &types.Value_StringValue{StringValue: network}}
	}

	return metadata
}

// updateClusterInc computes an envoy cluster assignment from the service shards.
// TODO: this code is incorrect. With config scoping, two sidecars can get
// a cluster of same name but with different set of endpoints. See the
// explanation below for more details
func (s *DiscoveryServer) updateClusterInc(push *model.PushContext, clusterName string,
	edsCluster *EdsCluster) error {

	var hostname model.Hostname
	var port int
	var subsetName string
	_, subsetName, hostname, port = model.ParseSubsetKey(clusterName)

	// TODO: BUG. this code is incorrect. With destination rule scoping
	// (public/private) as well as sidecar scopes allowing import of
	// specific destination rules, the destination rule for a given
	// namespace should be determined based on the sidecar scope or the
	// proxy's config namespace. As such, this code searches through all
	// destination rules, public and private and returns a completely
	// arbitrary destination rule's subset labels!
	labels := push.SubsetToLabels(subsetName, hostname)

	portMap, f := push.ServicePort2Name[string(hostname)]
	if !f {
		return s.updateCluster(push, clusterName, edsCluster)
	}
	svcPort, f := portMap.GetByPort(port)
	if !f {
		return s.updateCluster(push, clusterName, edsCluster)
	}

	// The service was never updated - do the full update
	se, f := s.EndpointShardsByService[string(hostname)]
	if !f {
		return s.updateCluster(push, clusterName, edsCluster)
	}

	cnt := 0
	localityEpMap := make(map[string]*endpoint.LocalityLbEndpoints)

	se.mutex.RLock()
	// The shards are updated independently, now need to filter and merge
	// for this cluster
	for _, endpoints := range se.Shards {
		for _, ep := range endpoints {
			if svcPort.Name != ep.ServicePortName {
				continue
			}
			// Port labels
			if !labels.HasSubsetOf(model.Labels(ep.Labels)) {
				continue
			}
			cnt++

			locLbEps, found := localityEpMap[ep.Locality]
			if !found {
				locLbEps = &endpoint.LocalityLbEndpoints{
					Locality: util.ConvertLocality(ep.Locality),
				}
				localityEpMap[ep.Locality] = locLbEps
			}
			if ep.EnvoyEndpoint == nil {
				ep.EnvoyEndpoint = buildEnvoyLbEndpoint(ep.UID, ep.Family, ep.Address, ep.EndpointPort, ep.Network)
			}
			locLbEps.LbEndpoints = append(locLbEps.LbEndpoints, *ep.EnvoyEndpoint)
		}
	}
	se.mutex.RUnlock()

	locEps := make([]endpoint.LocalityLbEndpoints, 0, len(localityEpMap))
	for _, locLbEps := range localityEpMap {
		locLbEps.LoadBalancingWeight = &types.UInt32Value{
			Value: uint32(len(locLbEps.LbEndpoints)),
		}
		locEps = append(locEps, *locLbEps)
	}
	// Normalize LoadBalancingWeight in range [1, 128]
	locEps = LoadBalancingWeightNormalize(locEps)

	if cnt == 0 {
		push.Add(model.ProxyStatusClusterNoInstances, clusterName, nil, "")
		//adsLog.Infof("EDS: no instances %s (host=%s ports=%v labels=%v)", clusterName, hostname, p, labels)
	}
	edsInstances.With(prometheus.Labels{"cluster": clusterName}).Set(float64(cnt))

	// There is a chance multiple goroutines will update the cluster at the same time.
	// This could be prevented by a lock - but because the update may be slow, it may be
	// better to accept the extra computations.
	// We still lock the access to the LoadAssignments.
	edsCluster.mutex.Lock()
	defer edsCluster.mutex.Unlock()

	edsCluster.LoadAssignment = &xdsapi.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   locEps,
	}
	if len(locEps) > 0 && edsCluster.NonEmptyTime.IsZero() {
		edsCluster.NonEmptyTime = time.Now()
	}
	return nil
}

// updateServiceShards will list the endpoints and create the shards.
// This is used to reconcile and to support non-k8s registries (until they migrate).
// Note that aggregated list is expensive (for large numbers) - we want to replace
// it with a model where DiscoveryServer keeps track of all endpoint registries
// directly, and calls them one by one.
func (s *DiscoveryServer) updateServiceShards(push *model.PushContext) error {

	// TODO: if ServiceDiscovery is aggregate, and all members support direct, use
	// the direct interface.
	var registries []aggregate.Registry
	if agg, ok := s.Env.ServiceDiscovery.(*aggregate.Controller); ok {
		registries = agg.GetRegistries()
	} else {
		registries = []aggregate.Registry{
			{
				ServiceDiscovery: s.Env.ServiceDiscovery,
			},
		}
	}

	// hostname --> service account
	svc2account := map[string]map[string]bool{}

	for _, registry := range registries {
		// Each registry acts as a shard - we don't want to combine them because some
		// may individually update their endpoints incrementally
		for _, svc := range push.Services(nil) {
			entries := []*model.IstioEndpoint{}
			hostname := string(svc.Hostname)
			for _, port := range svc.Ports {
				if port.Protocol == model.ProtocolUDP {
					continue
				}

				// This loses track of grouping (shards)
				endpoints, err := registry.InstancesByPort(svc.Hostname, port.Port, model.LabelsCollection{})
				if err != nil {
					return err
				}

				for _, ep := range endpoints {
					entries = append(entries, &model.IstioEndpoint{
						Family:          ep.Endpoint.Family,
						Address:         ep.Endpoint.Address,
						EndpointPort:    uint32(ep.Endpoint.Port),
						ServicePortName: port.Name,
						Labels:          ep.Labels,
						UID:             ep.Endpoint.UID,
						ServiceAccount:  ep.ServiceAccount,
						Network:         ep.Endpoint.Network,
						Locality:        ep.GetLocality(),
						LbWeight:        ep.Endpoint.LbWeight,
					})
					if ep.ServiceAccount != "" {
						account, f := svc2account[hostname]
						if !f {
							account = map[string]bool{}
							svc2account[hostname] = account
						}
						account[ep.ServiceAccount] = true
					}
				}
			}

			s.edsUpdate(registry.ClusterID, hostname, entries, true)
		}
	}

	s.mutex.Lock()
	for k, v := range svc2account {
		ep := s.EndpointShardsByService[k]
		ep.ServiceAccounts = v
	}
	s.mutex.Unlock()

	return nil
}

// updateCluster is called from the event (or global cache invalidation) to update
// the endpoints for the cluster.
func (s *DiscoveryServer) updateCluster(push *model.PushContext, clusterName string, edsCluster *EdsCluster) error {
	// TODO: should we lock this as well ? Once we move to event-based it may not matter.
	var locEps []endpoint.LocalityLbEndpoints
	direction, subsetName, hostname, port := model.ParseSubsetKey(clusterName)
	if direction == model.TrafficDirectionInbound ||
		direction == model.TrafficDirectionOutbound {
		labels := push.SubsetToLabels(subsetName, hostname)
		instances, err := s.Env.ServiceDiscovery.InstancesByPort(hostname, port, labels)
		if err != nil {
			adsLog.Errorf("endpoints for service cluster %q returned error %v", clusterName, err)
			totalXDSInternalErrors.Add(1)
			return err
		}
		if len(instances) == 0 {
			push.Add(model.ProxyStatusClusterNoInstances, clusterName, nil, "")
			adsLog.Debugf("EDS: cluster %q (host=%s ports=%v labels=%v) has no instances", clusterName, hostname, port, labels)
		}
		edsInstances.With(prometheus.Labels{"cluster": clusterName}).Set(float64(len(instances)))

		locEps = localityLbEndpointsFromInstances(instances)
	}

	for i := 0; i < len(locEps); i++ {
		locEps[i].LoadBalancingWeight = &types.UInt32Value{
			Value: uint32(len(locEps[i].LbEndpoints)),
		}
	}
	// There is a chance multiple goroutines will update the cluster at the same time.
	// This could be prevented by a lock - but because the update may be slow, it may be
	// better to accept the extra computations.
	// We still lock the access to the LoadAssignments.
	edsCluster.mutex.Lock()
	defer edsCluster.mutex.Unlock()

	// Normalize LoadBalancingWeight in range [1, 128]
	locEps = LoadBalancingWeightNormalize(locEps)

	edsCluster.LoadAssignment = &xdsapi.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   locEps,
	}
	if len(locEps) > 0 && edsCluster.NonEmptyTime.IsZero() {
		edsCluster.NonEmptyTime = time.Now()
	}
	return nil
}

// SvcUpdate is a callback from service discovery when service info changes.
func (s *DiscoveryServer) SvcUpdate(cluster, hostname string, ports map[string]uint32, rports map[uint32]string) {
	pc := s.globalPushContext()
	if cluster == "" {
		pl := model.PortList{}
		for k, v := range ports {
			pl = append(pl, &model.Port{
				Port: int(v),
				Name: k,
			})
		}
		pc.Mutex.Lock()
		pc.ServicePort2Name[hostname] = pl
		pc.Mutex.Unlock()
	}
	// TODO: for updates from other clusters, warn if they don't match primary.
}

// Update clusters for an incremental EDS push, and initiate the push.
// Only clusters that changed are updated/pushed.
func (s *DiscoveryServer) edsIncremental(version string, push *model.PushContext, edsUpdates map[string]struct{}) {
	adsLog.Infof("XDS:EDSInc Pushing %s Services: %v, "+
		"ConnectedEndpoints: %d", version, edsUpdates, adsClientCount())
	t0 := time.Now()

	// First update all cluster load assignments. This is computed for each cluster once per config change
	// instead of once per endpoint.
	edsClusterMutex.Lock()
	// Create a temp map to avoid locking the add/remove
	cMap := make(map[string]*EdsCluster, len(edsClusters))
	for k, v := range edsClusters {
		_, _, hostname, _ := model.ParseSubsetKey(k)
		if _, ok := edsUpdates[string(hostname)]; !ok {
			// Cluster was not updated, skip recomputing.
			continue
		}
		cMap[k] = v
	}
	edsClusterMutex.Unlock()

	// UpdateCluster updates the cluster with a mutex, this code is safe ( but computing
	// the update may be duplicated if multiple goroutines compute at the same time).
	// In general this code is called from the 'event' callback that is throttled.
	for clusterName, edsCluster := range cMap {
		if err := s.updateClusterInc(push, clusterName, edsCluster); err != nil {
			adsLog.Errorf("updateCluster failed with clusterName %s", clusterName)
		}
	}
	adsLog.Infof("Cluster init time %v %s", time.Since(t0), version)

	s.startPush(version, push, false, edsUpdates)
}

// WorkloadUpdate is called when workload labels/annotations are updated.
func (s *DiscoveryServer) WorkloadUpdate(id string, labels map[string]string, _ map[string]string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if labels == nil {
		// No push needed - the Endpoints object will also be triggered.
		delete(s.WorkloadsByID, id)
		return
	}
	w, f := s.WorkloadsByID[id]
	if !f {
		// First time this workload has been seen. Likely never connected, no need to
		// push
		s.WorkloadsByID[id] = &Workload{
			Labels: labels,
		}
		return
	}
	if reflect.DeepEqual(w.Labels, labels) {
		// No label change.
		return
	}

	w.Labels = labels
	// Label changes require recomputing the config.
	// TODO: we can do a push for the affected workload only, but we need to confirm
	// no other workload can be affected. Safer option is to fallback to full push.

	adsLog.Infof("Label change, full push %s ", id)
	s.ConfigUpdate(true)
}

// EDSUpdate computes destination address membership across all clusters and networks.
// This is the main method implementing EDS.
// It replaces InstancesByPort in model - instead of iterating over all endpoints it uses
// the hostname-keyed map. And it avoids the conversion from Endpoint to ServiceEntry to envoy
// on each step: instead the conversion happens once, when an endpoint is first discovered.
func (s *DiscoveryServer) EDSUpdate(shard, serviceName string,
	istioEndpoints []*model.IstioEndpoint) error {
	s.edsUpdate(shard, serviceName, istioEndpoints, false)
	return nil
}

// edsUpdate updates edsUpdates by shard, serviceName, IstioEndpoints,
// and requests a full/eds push.
func (s *DiscoveryServer) edsUpdate(shard, serviceName string,
	istioEndpoints []*model.IstioEndpoint, internal bool) {
	// edsShardUpdate replaces a subset (shard) of endpoints, as result of an incremental
	// update. The endpoint updates may be grouped by K8S clusters, other service registries
	// or by deployment. Multiple updates are debounced, to avoid too frequent pushes.
	// After debounce, the services are merged and pushed.
	s.mutex.Lock()
	defer s.mutex.Unlock()
	requireFull := false

	// Update the data structures for the service.
	// 1. Find the 'per service' data
	ep, f := s.EndpointShardsByService[serviceName]
	if !f {
		// This endpoint is for a service that was not previously loaded.
		// Return an error to force a full sync, which will also cause the
		// EndpointsShardsByService to be initialized with all services.
		ep = &EndpointShards{
			Shards:          map[string][]*model.IstioEndpoint{},
			ServiceAccounts: map[string]bool{},
		}
		s.EndpointShardsByService[serviceName] = ep
		if !internal {
			adsLog.Infof("Full push, new service %s", serviceName)
			requireFull = true
		}
	}

	// 2. Update data for the specific cluster. Each cluster gets independent
	// updates containing the full list of endpoints for the service in that cluster.
	for _, e := range istioEndpoints {
		if e.ServiceAccount != "" {
			_, f = ep.ServiceAccounts[e.ServiceAccount]
			if !f && !internal {
				// The entry has a service account that was not previously associated.
				// Requires a CDS push and full sync.
				adsLog.Infof("Endpoint updating service account %s %s", e.ServiceAccount, serviceName)
				requireFull = true
			}
		}
	}
	ep.mutex.Lock()
	ep.Shards[shard] = istioEndpoints
	ep.mutex.Unlock()
	s.edsUpdates[serviceName] = struct{}{}

	// for internal update: this called by DiscoveryServer.Push --> updateServiceShards,
	// no need to trigger push here.
	// It is done in DiscoveryServer.Push --> AdsPushAll
	if !internal {
		if requireFull {
			s.ConfigUpdate(true)
		} else {
			s.ConfigUpdate(false)
		}
	}
}

// LocalityLbEndpointsFromInstances returns a list of Envoy v2 LocalityLbEndpoints.
// Envoy v2 Endpoints are constructed from Pilot's older data structure involving
// model.ServiceInstance objects. Envoy expects the endpoints grouped by zone, so
// a map is created - in new data structures this should be part of the model.
func localityLbEndpointsFromInstances(instances []*model.ServiceInstance) []endpoint.LocalityLbEndpoints {
	localityEpMap := make(map[string]*endpoint.LocalityLbEndpoints)
	for _, instance := range instances {
		lbEp, err := networkEndpointToEnvoyEndpoint(&instance.Endpoint)
		if err != nil {
			adsLog.Errorf("EDS: unexpected pilot model endpoint v1 to v2 conversion: %v", err)
			totalXDSInternalErrors.Add(1)
			continue
		}
		locality := instance.GetLocality()
		locLbEps, found := localityEpMap[locality]
		if !found {
			locLbEps = &endpoint.LocalityLbEndpoints{
				Locality: util.ConvertLocality(locality),
			}
			localityEpMap[locality] = locLbEps
		}
		locLbEps.LbEndpoints = append(locLbEps.LbEndpoints, *lbEp)
	}
	out := make([]endpoint.LocalityLbEndpoints, 0, len(localityEpMap))
	for _, locLbEps := range localityEpMap {
		out = append(out, *locLbEps)
	}
	return out
}

func connectionID(node string) string {
	id := atomic.AddInt64(&connectionNumber, 1)
	return node + "-" + strconv.FormatInt(id, 10)
}

// pushEds is pushing EDS updates for a single connection. Called the first time
// a client connects, for incremental updates and for full periodic updates.
func (s *DiscoveryServer) pushEds(push *model.PushContext, con *XdsConnection, edsUpdatedServices map[string]struct{}) error {
	loadAssignments := []*xdsapi.ClusterLoadAssignment{}
	emptyClusters := 0
	endpoints := 0
	empty := []string{}

	for _, clusterName := range con.Clusters {

		_, _, hostname, _ := model.ParseSubsetKey(clusterName)
		if edsUpdatedServices != nil {
			if _, ok := edsUpdatedServices[string(hostname)]; !ok {
				// Cluster was not updated, skip recomputing.
				continue
			}
		}

		c := s.getEdsCluster(clusterName)
		if c == nil {
			totalXDSInternalErrors.Add(1)
			adsLog.Errorf("cluster %s was nil skipping it.", clusterName)
			continue
		}

		l := loadAssignment(c)
		if l == nil { // fresh cluster
			if err := s.updateCluster(push, clusterName, c); err != nil {
				adsLog.Errorf("error returned from updateCluster for cluster name %s, skipping it.", clusterName)
				totalXDSInternalErrors.Add(1)
				continue
			}
			l = loadAssignment(c)
		}

		// If networks are set (by default they aren't) apply the Split Horizon
		// EDS filter on the endpoints
		if s.Env.MeshNetworks != nil && len(s.Env.MeshNetworks.Networks) > 0 {
			endpoints := EndpointsByNetworkFilter(l.Endpoints, con, s.Env)
			endpoints = LoadBalancingWeightNormalize(endpoints)
			filteredCLA := &xdsapi.ClusterLoadAssignment{
				ClusterName: l.ClusterName,
				Endpoints:   endpoints,
				Policy:      l.Policy,
			}
			l = filteredCLA
		}

		endpoints += len(l.Endpoints)
		if len(l.Endpoints) == 0 {
			emptyClusters++
			empty = append(empty, clusterName)
		}
		loadAssignments = append(loadAssignments, l)
	}

	response := endpointDiscoveryResponse(loadAssignments)
	err := con.send(response)
	if err != nil {
		adsLog.Warnf("EDS: Send failure %s: %v", con.ConID, err)
		pushes.With(prometheus.Labels{"type": "eds_senderr"}).Add(1)
		return err
	}
	pushes.With(prometheus.Labels{"type": "eds"}).Add(1)

	if edsUpdatedServices == nil {
		adsLog.Debugf("EDS: PUSH for %s clusters %d endpoints %d empty %d",
			con.ConID, len(con.Clusters), endpoints, emptyClusters)
	} else {
		adsLog.Debugf("EDS: INC PUSH for %s clusters %d endpoints %d empty %d",
			con.ConID, len(con.Clusters), endpoints, emptyClusters)
	}
	return nil
}

// addEdsCon will track the eds connection with clusters, for optimized event-based push and debug
func (s *DiscoveryServer) addEdsCon(clusterName string, node string, connection *XdsConnection) {

	c := s.getOrAddEdsCluster(clusterName)
	// TODO: left the code here so we can skip sending the already-sent clusters.
	// See comments in ads - envoy keeps adding one cluster to the list (this seems new
	// previous version sent all the clusters from CDS in bulk).

	//c.mutex.Lock()
	//existing := c.EdsClients[node]
	//c.mutex.Unlock()
	//
	//// May replace an existing connection: this happens when Envoy adds more clusters
	//// one by one, creating new grpc requests each time it adds one more cluster.
	//if existing != nil {
	//	log.Warnf("Replacing existing connection %s %s old: %s", clusterName, node, existing.ConID)
	//}
	c.mutex.Lock()
	c.EdsClients[node] = connection
	c.mutex.Unlock()
}

// getEdsCluster returns a cluster.
func (s *DiscoveryServer) getEdsCluster(clusterName string) *EdsCluster {
	// separate method only to have proper lock.
	edsClusterMutex.RLock()
	defer edsClusterMutex.RUnlock()
	return edsClusters[clusterName]
}

func (s *DiscoveryServer) getOrAddEdsCluster(clusterName string) *EdsCluster {
	edsClusterMutex.Lock()
	defer edsClusterMutex.Unlock()

	c := edsClusters[clusterName]
	if c == nil {
		c = &EdsCluster{discovery: s,
			EdsClients: map[string]*XdsConnection{},
			FirstUse:   time.Now(),
		}
		edsClusters[clusterName] = c
	}
	return c
}

// removeEdsCon is called when a gRPC stream is closed, for each cluster that was watched by the
// stream. As of 0.7 envoy watches a single cluster per gprc stream.
func (s *DiscoveryServer) removeEdsCon(clusterName string, node string, connection *XdsConnection) {
	c := s.getEdsCluster(clusterName)
	if c == nil {
		adsLog.Warnf("EDS: missing cluster %s", clusterName)
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	oldcon := c.EdsClients[node]
	if oldcon == nil {
		adsLog.Warnf("EDS: Envoy restart %s %v, cleanup old connection missing %v", node, connection.PeerAddr, c.EdsClients)
		return
	}
	if oldcon != connection {
		adsLog.Infof("EDS: Envoy restart %s %v, cleanup old connection %v", node, connection.PeerAddr, oldcon.PeerAddr)
		return
	}
	delete(c.EdsClients, node)
	if len(c.EdsClients) == 0 {
		edsClusterMutex.Lock()
		defer edsClusterMutex.Unlock()
		// This happens when a previously used cluster is no longer watched by any
		// sidecar. It should not happen very often - normally all clusters are sent
		// in CDS requests to all sidecars. It may happen if all connections are closed.
		adsLog.Debugf("EDS: remove unwatched cluster node=%s cluster=%s", node, clusterName)
		delete(edsClusters, clusterName)
	}
}

func endpointDiscoveryResponse(loadAssignments []*xdsapi.ClusterLoadAssignment) *xdsapi.DiscoveryResponse {
	out := &xdsapi.DiscoveryResponse{
		TypeUrl: EndpointType,
		// Pilot does not really care for versioning. It always supplies what's currently
		// available to it, irrespective of whether Envoy chooses to accept or reject EDS
		// responses. Pilot believes in eventual consistency and that at some point, Envoy
		// will begin seeing results it deems to be good.
		VersionInfo: versionInfo(),
		Nonce:       nonce(),
	}
	for _, loadAssignment := range loadAssignments {
		resource, _ := types.MarshalAny(loadAssignment)
		out.Resources = append(out.Resources, *resource)
	}

	return out
}
