// Copyright 2017 Istio Authors
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

package v1alpha3

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	fileaccesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	http_conn "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	tcp_proxy "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/envoyproxy/go-control-plane/envoy/type"
	xdsutil "github.com/envoyproxy/go-control-plane/pkg/util"
	google_protobuf "github.com/gogo/protobuf/types"
	"github.com/prometheus/client_golang/prometheus"

	meshconfig "istio.io/api/mesh/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/plugin"
	"istio.io/istio/pilot/pkg/networking/util"
	"istio.io/istio/pkg/log"
	"istio.io/istio/pkg/proto"
)

const (
	envoyListenerTLSInspector = "envoy.listener.tls_inspector"

	// RDSHttpProxy is the special name for HTTP PROXY route
	RDSHttpProxy = "http_proxy"

	// VirtualListenerName is the name for traffic capture listener
	VirtualListenerName = "virtual"

	// WildcardAddress binds to all IP addresses
	WildcardAddress = "0.0.0.0"

	// LocalhostAddress for local binding
	LocalhostAddress = "127.0.0.1"

	// EnvoyTextLogFormat format for envoy text based access logs
	EnvoyTextLogFormat = "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% " +
		"%PROTOCOL%\" %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT% " +
		"%DURATION% %RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)% \"%REQ(X-FORWARDED-FOR)%\" " +
		"\"%REQ(USER-AGENT)%\" \"%REQ(X-REQUEST-ID)%\" \"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\" " +
		"%UPSTREAM_CLUSTER% %UPSTREAM_LOCAL_ADDRESS% %DOWNSTREAM_LOCAL_ADDRESS% " +
		"%DOWNSTREAM_REMOTE_ADDRESS% %REQUESTED_SERVER_NAME%\n"

	// EnvoyServerName for istio's envoy
	EnvoyServerName = "istio-envoy"
)

var (
	// EnvoyJSONLogFormat map of values for envoy json based access logs
	EnvoyJSONLogFormat = &google_protobuf.Struct{
		Fields: map[string]*google_protobuf.Value{
			"start_time":                &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%START_TIME%"}},
			"method":                    &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%START_TIME%"}},
			"path":                      &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%"}},
			"protocol":                  &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%PROTOCOL%"}},
			"response_code":             &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%RESPONSE_CODE%"}},
			"response_flags":            &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%RESPONSE_FLAGS%"}},
			"bytes_received":            &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%BYTES_RECEIVED%"}},
			"bytes_sent":                &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%BYTES_SENT%"}},
			"duration":                  &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%DURATION%"}},
			"upstream_service_time":     &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%"}},
			"x_forwarded_for":           &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%REQ(X-FORWARDED-FOR)%"}},
			"user_agent":                &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%REQ(USER-AGENT)%"}},
			"request_id":                &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%REQ(X-REQUEST-ID)%"}},
			"authority":                 &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%REQ(:AUTHORITY)%"}},
			"upstream_host":             &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%UPSTREAM_HOST%"}},
			"upstream_cluster":          &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%UPSTREAM_CLUSTER%"}},
			"upstream_local_address":    &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%UPSTREAM_LOCAL_ADDRESS%"}},
			"downstream_local_address":  &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%DOWNSTREAM_LOCAL_ADDRESS%"}},
			"downstream_remote_address": &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%DOWNSTREAM_REMOTE_ADDRESS%"}},
			"requested_server_name":     &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: "%REQUESTED_SERVER_NAME%"}},
		},
	}
)

func buildAccessLog(fl *fileaccesslog.FileAccessLog, env *model.Environment) {
	switch env.Mesh.AccessLogEncoding {
	case meshconfig.MeshConfig_TEXT:
		formatString := EnvoyTextLogFormat
		if env.Mesh.AccessLogFormat != "" {
			formatString = env.Mesh.AccessLogFormat
		}
		fl.AccessLogFormat = &fileaccesslog.FileAccessLog_Format{
			Format: formatString,
		}
	case meshconfig.MeshConfig_JSON:
		var jsonLog *google_protobuf.Struct
		// TODO potential optimization to avoid recomputing the user provided format for every listener
		// mesh AccessLogFormat field could change so need a way to have a cached value that can be cleared
		// on changes
		if env.Mesh.AccessLogFormat != "" {
			jsonFields := map[string]string{}
			err := json.Unmarshal([]byte(env.Mesh.AccessLogFormat), &jsonFields)
			if err == nil {
				jsonLog = &google_protobuf.Struct{
					Fields: make(map[string]*google_protobuf.Value, len(jsonFields)),
				}
				fmt.Println(jsonFields)
				for key, value := range jsonFields {
					jsonLog.Fields[key] = &google_protobuf.Value{Kind: &google_protobuf.Value_StringValue{StringValue: value}}
				}
			} else {
				fmt.Println(env.Mesh.AccessLogFormat)
				log.Errorf("error parsing provided json log format, default log format will be used: %v", err)
			}
		}
		if jsonLog == nil {
			jsonLog = EnvoyJSONLogFormat
		}
		fl.AccessLogFormat = &fileaccesslog.FileAccessLog_JsonFormat{
			JsonFormat: jsonLog,
		}
	default:
		log.Warnf("unsupported access log format %v", env.Mesh.AccessLogEncoding)
	}
}

var (
	// TODO: gauge should be reset on refresh, not the best way to represent errors but better
	// than nothing.
	// TODO: add dimensions - namespace of rule, service, rule name
	invalidOutboundListeners = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pilot_invalid_out_listeners",
		Help: "Number of invalid outbound listeners.",
	})
)

func init() {
	prometheus.MustRegister(invalidOutboundListeners)
}

// ListenersALPNProtocols denotes the the list of ALPN protocols that the listener
// should expose
var ListenersALPNProtocols = []string{"h2", "http/1.1"}

// BuildListeners produces a list of listeners and referenced clusters for all proxies
func (configgen *ConfigGeneratorImpl) BuildListeners(env *model.Environment, node *model.Proxy, push *model.PushContext) ([]*xdsapi.Listener, error) {
	switch node.Type {
	case model.SidecarProxy:
		return configgen.buildSidecarListeners(env, node, push)
	case model.Router, model.Ingress:
		return configgen.buildGatewayListeners(env, node, push)
	}
	return nil, nil
}

// buildSidecarListeners produces a list of listeners for sidecar proxies
func (configgen *ConfigGeneratorImpl) buildSidecarListeners(env *model.Environment, node *model.Proxy,
	push *model.PushContext) ([]*xdsapi.Listener, error) {

	mesh := env.Mesh

	proxyInstances, err := env.GetProxyServiceInstances(node)
	if err != nil {
		return nil, err
	}

	services := push.Services(node)
	sidecarScope := push.GetSidecarScope(node, proxyInstances)

	listeners := make([]*xdsapi.Listener, 0)

	if mesh.ProxyListenPort > 0 {
		inbound := configgen.buildSidecarInboundListeners(env, node, push, proxyInstances, sidecarScope)
		outbound := configgen.buildSidecarOutboundListeners(env, node, push, proxyInstances, services, sidecarScope)

		listeners = append(listeners, inbound...)
		listeners = append(listeners, outbound...)

		// Let ServiceDiscovery decide which IP and Port are used for management if
		// there are multiple IPs
		mgmtListeners := make([]*xdsapi.Listener, 0)
		for _, ip := range node.IPAddresses {
			managementPorts := env.ManagementPorts(ip)
			management := buildSidecarInboundMgmtListeners(node, env, managementPorts, ip)
			mgmtListeners = append(mgmtListeners, management...)
		}

		// If management listener port and service port are same, bad things happen
		// when running in kubernetes, as the probes stop responding. So, append
		// non overlapping listeners only.
		for i := range mgmtListeners {
			m := mgmtListeners[i]
			l := util.GetByAddress(listeners, m.Address.String())
			if l != nil {
				log.Warnf("Omitting listener for management address %s (%s) due to collision with service listener %s (%s)",
					m.Name, m.Address.String(), l.Name, l.Address.String())
				continue
			}
			listeners = append(listeners, m)
		}

		// We need a passthrough filter to fill in the filter stack for orig_dst listener
		passthroughTCPProxy := &tcp_proxy.TcpProxy{
			StatPrefix:       util.PassthroughCluster,
			ClusterSpecifier: &tcp_proxy.TcpProxy_Cluster{Cluster: util.PassthroughCluster},
		}

		var transparent *google_protobuf.BoolValue
		if mode := node.Metadata["INTERCEPTION_MODE"]; mode == "TPROXY" {
			transparent = proto.BoolTrue
		}

		// add an extra listener that binds to the port that is the recipient of the iptables redirect
		listeners = append(listeners, &xdsapi.Listener{
			Name:           VirtualListenerName,
			Address:        util.BuildAddress(WildcardAddress, uint32(mesh.ProxyListenPort)),
			Transparent:    transparent,
			UseOriginalDst: proto.BoolTrue,
			FilterChains: []listener.FilterChain{
				{
					Filters: []listener.Filter{
						{
							Name: xdsutil.TCPProxy,
							ConfigType: &listener.Filter_Config{
								Config: util.MessageToStruct(passthroughTCPProxy),
							},
						},
					},
				},
			},
		})
	}

	// enable HTTP PROXY port if necessary; this will add an RDS route for this port
	if mesh.ProxyHttpPort > 0 {
		useRemoteAddress := false
		traceOperation := http_conn.EGRESS
		listenAddress := LocalhostAddress

		if node.Type == model.Router {
			useRemoteAddress = true
			traceOperation = http_conn.INGRESS
			listenAddress = WildcardAddress
		}

		opts := buildListenerOpts{
			env:            env,
			proxy:          node,
			proxyInstances: proxyInstances,
			ip:             listenAddress,
			port:           int(mesh.ProxyHttpPort),
			filterChainOpts: []*filterChainOpts{{
				httpOpts: &httpListenerOpts{
					rds:              RDSHttpProxy,
					useRemoteAddress: useRemoteAddress,
					direction:        traceOperation,
					connectionManager: &http_conn.HttpConnectionManager{
						HttpProtocolOptions: &core.Http1ProtocolOptions{
							AllowAbsoluteUrl: proto.BoolTrue,
						},
					},
				},
			}},
			bindToPort:      true,
			skipUserFilters: true,
		}
		l := buildListener(opts)
		// TODO: plugins for HTTP_PROXY mode, envoyfilter needs another listener match for SIDECAR_HTTP_PROXY
		// there is no mixer for http_proxy
		mutable := &plugin.MutableObjects{
			Listener:     l,
			FilterChains: []plugin.FilterChain{{}},
		}
		pluginParams := &plugin.InputParams{
			ListenerProtocol: plugin.ListenerProtocolHTTP,
			ListenerCategory: networking.EnvoyFilter_ListenerMatch_SIDECAR_OUTBOUND,
			Env:              env,
			Node:             node,
			ProxyInstances:   proxyInstances,
			Push:             push,
		}
		if err := buildCompleteFilterChain(pluginParams, mutable, opts); err != nil {
			log.Warna("buildSidecarListeners ", err.Error())
		} else {
			listeners = append(listeners, l)
		}
		// TODO: need inbound listeners in HTTP_PROXY case, with dedicated ingress listener.
	}

	return listeners, nil
}

// buildSidecarInboundListeners creates listeners for the server-side (inbound)
// configuration for co-located service proxyInstances.
func (configgen *ConfigGeneratorImpl) buildSidecarInboundListeners(env *model.Environment, node *model.Proxy, push *model.PushContext,
	proxyInstances []*model.ServiceInstance, _ *model.SidecarScope) []*xdsapi.Listener {

	var listeners []*xdsapi.Listener
	listenerMap := make(map[string]*model.ServiceInstance)
	// inbound connections/requests are redirected to the endpoint address but appear to be sent
	// to the service address.
	for _, instance := range proxyInstances {
		endpoint := instance.Endpoint
		protocol := endpoint.ServicePort.Protocol

		// Local service instances can be accessed through one of three
		// addresses: localhost, endpoint IP, and service
		// VIP. Localhost bypasses the proxy and doesn't need any TCP
		// route config. Endpoint IP is handled below and Service IP is handled
		// by outbound routes.
		// Traffic sent to our service VIP is redirected by remote
		// services' kubeproxy to our specific endpoint IP.
		listenerOpts := buildListenerOpts{
			env:            env,
			proxy:          node,
			proxyInstances: proxyInstances,
			ip:             endpoint.Address,
			port:           endpoint.Port,
		}

		listenerMapKey := fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port)
		if old, exists := listenerMap[listenerMapKey]; exists {
			push.Add(model.ProxyStatusConflictInboundListener, node.ID, node,
				fmt.Sprintf("Rejected %s, used %s for %s", instance.Service.Hostname, old.Service.Hostname, listenerMapKey))
			// Skip building listener for the same ip port
			continue
		}
		allChains := []plugin.FilterChain{}
		var httpOpts *httpListenerOpts
		var tcpNetworkFilters []listener.Filter
		listenerProtocol := plugin.ModelProtocolToListenerProtocol(protocol)
		pluginParams := &plugin.InputParams{
			ListenerProtocol: listenerProtocol,
			ListenerCategory: networking.EnvoyFilter_ListenerMatch_SIDECAR_INBOUND,
			Env:              env,
			Node:             node,
			ProxyInstances:   proxyInstances,
			ServiceInstance:  instance,
			Port:             endpoint.ServicePort,
			Push:             push,
		}
		switch listenerProtocol {
		case plugin.ListenerProtocolHTTP:
			httpOpts = &httpListenerOpts{
				routeConfig:      configgen.buildSidecarInboundHTTPRouteConfig(env, node, push, instance),
				rds:              "", // no RDS for inbound traffic
				useRemoteAddress: false,
				direction:        http_conn.INGRESS,
				connectionManager: &http_conn.HttpConnectionManager{
					// Append and forward client cert to backend.
					ForwardClientCertDetails: http_conn.APPEND_FORWARD,
					SetCurrentClientCertDetails: &http_conn.HttpConnectionManager_SetCurrentClientCertDetails{
						Subject: &google_protobuf.BoolValue{Value: true},
						Uri:     true,
						Dns:     true,
					},
					ServerName: EnvoyServerName,
				},
			}
			// See https://github.com/grpc/grpc-web/tree/master/net/grpc/gateway/examples/helloworld#configure-the-proxy
			if endpoint.ServicePort.Protocol.IsHTTP2() {
				httpOpts.connectionManager.Http2ProtocolOptions = &core.Http2ProtocolOptions{}
				if endpoint.ServicePort.Protocol == model.ProtocolGRPCWeb {
					httpOpts.addGRPCWebFilter = true
				}
			}

		case plugin.ListenerProtocolTCP:
			tcpNetworkFilters = buildInboundNetworkFilters(env, node, instance)

		default:
			log.Warnf("Unsupported inbound protocol %v for port %#v", protocol, endpoint.ServicePort)
			continue
		}

		for _, p := range configgen.Plugins {
			chains := p.OnInboundFilterChains(pluginParams)
			if len(chains) == 0 {
				continue
			}
			if len(allChains) != 0 {
				log.Warnf("Found two plugin setups inbound filter chains for listeners, FilterChainMatch may not work as intended!")
			}
			allChains = append(allChains, chains...)
		}
		// Construct the default filter chain.
		if len(allChains) == 0 {
			log.Infof("Use default filter chain for %v", endpoint)
			// add one empty entry to the list so we generate a default listener below
			allChains = []plugin.FilterChain{{}}
		}
		for _, chain := range allChains {
			listenerOpts.filterChainOpts = append(listenerOpts.filterChainOpts, &filterChainOpts{
				httpOpts:        httpOpts,
				networkFilters:  tcpNetworkFilters,
				tlsContext:      chain.TLSContext,
				match:           chain.FilterChainMatch,
				listenerFilters: chain.ListenerFilters,
			})
		}

		// call plugins
		l := buildListener(listenerOpts)
		mutable := &plugin.MutableObjects{
			Listener:     l,
			FilterChains: make([]plugin.FilterChain, len(l.FilterChains)),
		}
		for _, p := range configgen.Plugins {
			if err := p.OnInboundListener(pluginParams, mutable); err != nil {
				log.Warn(err.Error())
			}
		}
		// Filters are serialized one time into an opaque struct once we have the complete list.
		if err := buildCompleteFilterChain(pluginParams, mutable, listenerOpts); err != nil {
			log.Warna("buildSidecarInboundListeners ", err.Error())
		} else {
			listeners = append(listeners, mutable.Listener)
			listenerMap[listenerMapKey] = instance
		}
	}
	return listeners
}

type listenerEntry struct {
	// TODO: Clean this up
	services    []*model.Service
	servicePort *model.Port
	listener    *xdsapi.Listener
}

func protocolName(p model.Protocol) string {
	switch plugin.ModelProtocolToListenerProtocol(p) {
	case plugin.ListenerProtocolHTTP:
		return "HTTP"
	case plugin.ListenerProtocolTCP:
		return "TCP"
	default:
		return "UNKNOWN"
	}
}

type outboundListenerConflict struct {
	metric          *model.PushMetric
	node            *model.Proxy
	listenerName    string
	currentProtocol model.Protocol
	currentServices []*model.Service
	newHostname     model.Hostname
	newProtocol     model.Protocol
}

func (c outboundListenerConflict) addMetric(push *model.PushContext) {
	currentHostnames := make([]string, len(c.currentServices))
	for i, s := range c.currentServices {
		currentHostnames[i] = string(s.Hostname)
	}
	concatHostnames := strings.Join(currentHostnames, ",")
	push.Add(c.metric,
		c.listenerName,
		c.node,
		fmt.Sprintf("Listener=%s Accepted%s=%s Rejected%s=%s %sServices=%d",
			c.listenerName,
			protocolName(c.currentProtocol),
			concatHostnames,
			protocolName(c.newProtocol),
			c.newHostname,
			protocolName(c.currentProtocol),
			len(c.currentServices)))
}

// buildSidecarOutboundListeners generates http and tcp listeners for outbound connections from the service instance
// TODO(github.com/istio/pilot/issues/237)
//
// Sharing tcp_proxy and http_connection_manager filters on the same port for
// different destination services doesn't work with Envoy (yet). When the
// tcp_proxy filter's route matching fails for the http service the connection
// is closed without falling back to the http_connection_manager.
//
// Temporary workaround is to add a listener for each service IP that requires
// TCP routing
//
// Connections to the ports of non-load balanced services are directed to
// the connection's original destination. This avoids costly queries of instance
// IPs and ports, but requires that ports of non-load balanced service be unique.
func (configgen *ConfigGeneratorImpl) buildSidecarOutboundListeners(env *model.Environment, node *model.Proxy,
	push *model.PushContext, proxyInstances []*model.ServiceInstance,
	services []*model.Service, sidecarScope *model.SidecarScope) []*xdsapi.Listener {

	var proxyLabels model.LabelsCollection
	for _, w := range proxyInstances {
		proxyLabels = append(proxyLabels, w.Labels)
	}

	meshGateway := map[string]bool{model.IstioMeshGateway: true}
	configs := push.VirtualServices(node, meshGateway)

	var tcpListeners, httpListeners []*xdsapi.Listener
	// For conflict resolution
	listenerMap := make(map[string]*listenerEntry)

	// The sidecarConfig if provided could filter the list of
	// services/virtual services that we need to process. It could also
	// define one or more listeners with specific ports. Once we generate
	// listeners for these user specified ports, we will auto generate
	// configs for other ports if and only if the sidecarConfig has an
	// egressListener on wildcard port.
	//
	// Validation will ensure that we have utmost one wildcard egress listener
	// occurring in the end

	var catchAllListener *model.IstioListenerWrapper
	var importedServices []*model.Service
	var importedConfigs []model.Config
	// Add listeners based on the config in the sidecar.EgressListeners if
	// no Sidecar CRD is provided for this config namespace,
	// push.SidecarScope will generate a default catch all egress listener.
	for _, egressListener := range sidecarScope.EgressListeners {
		if egressListener.IstioListener != nil &&
			egressListener.IstioListener.Port != nil {
			// We have a non catch all listener on some user specified port
			// The user specified port may or may not match a service port.
			// If it does not match any service port, then we expect the
			// user to provide a virtualService that will route to a proper
			// Service. This is the reason why we can't reuse the big
			// forloop logic below as it iterates over all services and
			// their service ports.

			// TODO: complete implementation
			continue
		}

		// This is a catch all egress listener. This should be the last
		// egress listener in the sidecar Scope.
		catchAllListener = egressListener
		break
	}

	if catchAllListener == nil {
		goto validateListeners
	}

	// Only import services and virtualServices required by this listener
	importedServices = catchAllListener.SelectServices(services)
	importedConfigs = catchAllListener.SelectVirtualServices(configs)

	// Control reaches this stage when we need to build a catch all egress
	// listener. We need to generate a listener for every unique service
	// port across all imported services, if and only if this port was not
	// specified in any of the preceding listeners from the sidecarScope.
	// TODO: Implement the logic for ignoring service ports processed earlier.
	for _, service := range importedServices {
		for _, servicePort := range service.Ports {
			listenAddress := WildcardAddress
			var destinationIPAddress string
			var listenerMapKey string
			var currentListenerEntry *listenerEntry
			listenerOpts := buildListenerOpts{
				env:            env,
				proxy:          node,
				proxyInstances: proxyInstances,
				ip:             WildcardAddress,
				port:           servicePort.Port,
			}

			pluginParams := &plugin.InputParams{
				ListenerProtocol: plugin.ModelProtocolToListenerProtocol(servicePort.Protocol),
				ListenerCategory: networking.EnvoyFilter_ListenerMatch_SIDECAR_OUTBOUND,
				Env:              env,
				Node:             node,
				ProxyInstances:   proxyInstances,
				Service:          service,
				Port:             servicePort,
				Push:             push,
			}
			switch pluginParams.ListenerProtocol {
			case plugin.ListenerProtocolHTTP:
				listenerMapKey = fmt.Sprintf("%s:%d", listenAddress, servicePort.Port)
				var exists bool
				// Check if this HTTP listener conflicts with an existing wildcard TCP listener
				// i.e. one of NONE resolution type, since we collapse all HTTP listeners into
				// a single 0.0.0.0:port listener and use vhosts to distinguish individual http
				// services in that port
				if currentListenerEntry, exists = listenerMap[listenerMapKey]; exists {
					if !currentListenerEntry.servicePort.Protocol.IsHTTP() {
						outboundListenerConflict{
							metric:          model.ProxyStatusConflictOutboundListenerTCPOverHTTP,
							node:            node,
							listenerName:    listenerMapKey,
							currentServices: currentListenerEntry.services,
							currentProtocol: currentListenerEntry.servicePort.Protocol,
							newHostname:     service.Hostname,
							newProtocol:     servicePort.Protocol,
						}.addMetric(push)
					}
					// Skip building listener for the same http port
					currentListenerEntry.services = append(currentListenerEntry.services, service)
					continue
				}

				listenerOpts.filterChainOpts = []*filterChainOpts{{
					httpOpts: &httpListenerOpts{
						rds:              fmt.Sprintf("%d", servicePort.Port),
						useRemoteAddress: false,
						direction:        http_conn.EGRESS,
					},
				}}
			case plugin.ListenerProtocolTCP:
				// Determine the listener address
				// we listen on the service VIP if and only
				// if the address is an IP address. If its a CIDR, we listen on
				// 0.0.0.0, and setup a filter chain match for the CIDR range.
				// As a small optimization, CIDRs with /32 prefix will be converted
				// into listener address so that there is a dedicated listener for this
				// ip:port. This will reduce the impact of a listener reload

				svcListenAddress := service.GetServiceAddressForProxy(node)
				// We should never get an empty address.
				// This is a safety guard, in case some platform adapter isn't doing things
				// properly
				if len(svcListenAddress) > 0 {
					if !strings.Contains(svcListenAddress, "/") {
						listenAddress = svcListenAddress
					} else {
						// Address is a CIDR. Fall back to 0.0.0.0 and
						// filter chain match
						destinationIPAddress = svcListenAddress
					}
				}

				listenerMapKey = fmt.Sprintf("%s:%d", listenAddress, servicePort.Port)
				var exists bool
				// Check if this TCP listener conflicts with an existing HTTP listener on 0.0.0.0:Port
				if currentListenerEntry, exists = listenerMap[listenerMapKey]; exists {
					// Check for port collisions between TCP/TLS and HTTP.
					// If configured correctly, TCP/TLS ports may not collide.
					// We'll need to do additional work to find out if there is a collision within TCP/TLS.
					if !currentListenerEntry.servicePort.Protocol.IsTCP() {
						outboundListenerConflict{
							metric:          model.ProxyStatusConflictOutboundListenerHTTPOverTCP,
							node:            node,
							listenerName:    listenerMapKey,
							currentServices: currentListenerEntry.services,
							currentProtocol: currentListenerEntry.servicePort.Protocol,
							newHostname:     service.Hostname,
							newProtocol:     servicePort.Protocol,
						}.addMetric(push)
						continue
					}
					// WE have a collision with another TCP port.
					// This can happen only if the service is listening on 0.0.0.0:<port>
					// which is the case for headless services, or non-k8s services that do not have a VIP.
					// Unfortunately we won't know if this is a real conflict or not
					// until we process the VirtualServices, etc.
					// The conflict resolution is done later in this code
				}

				listenerOpts.filterChainOpts = buildSidecarOutboundTCPTLSFilterChainOpts(env, node, push, importedConfigs,
					destinationIPAddress, service, servicePort, proxyLabels, meshGateway)
			default:
				// UDP or other protocols: no need to log, it's too noisy
				continue
			}

			// Even if we have a non empty current listener, lets build the new listener with the filter chains
			// In the end, we will merge the filter chains

			// call plugins
			listenerOpts.ip = listenAddress
			l := buildListener(listenerOpts)
			mutable := &plugin.MutableObjects{
				Listener:     l,
				FilterChains: make([]plugin.FilterChain, len(l.FilterChains)),
			}

			for _, p := range configgen.Plugins {
				if err := p.OnOutboundListener(pluginParams, mutable); err != nil {
					log.Warn(err.Error())
				}
			}

			// Filters are serialized one time into an opaque struct once we have the complete list.
			if err := buildCompleteFilterChain(pluginParams, mutable, listenerOpts); err != nil {
				log.Warna("buildSidecarOutboundListeners: ", err.Error())
				continue
			}

			// TODO(rshriram) merge multiple identical filter chains with just a single destination CIDR based
			// filter chain matche, into a single filter chain and array of destinationcidr matches

			// We checked TCP over HTTP, and HTTP over TCP conflicts above.
			// The code below checks for TCP over TCP conflicts and merges listeners
			if currentListenerEntry != nil {
				// merge the newly built listener with the existing listener
				// if and only if the filter chains have distinct conditions
				// Extract the current filter chain matches
				// For every new filter chain match being added, check if any previous match is same
				// if so, skip adding this filter chain with a warning
				// This is very unoptimized.
				newFilterChains := make([]listener.FilterChain, 0,
					len(currentListenerEntry.listener.FilterChains)+len(mutable.Listener.FilterChains))
				newFilterChains = append(newFilterChains, currentListenerEntry.listener.FilterChains...)
				for _, incomingFilterChain := range mutable.Listener.FilterChains {
					conflictFound := false

				compareWithExisting:
					for _, existingFilterChain := range currentListenerEntry.listener.FilterChains {
						if existingFilterChain.FilterChainMatch == nil {
							// This is a catch all filter chain.
							// We can only merge with a non-catch all filter chain
							// Else mark it as conflict
							if incomingFilterChain.FilterChainMatch == nil {
								conflictFound = true
								outboundListenerConflict{
									metric:          model.ProxyStatusConflictOutboundListenerTCPOverTCP,
									node:            node,
									listenerName:    listenerMapKey,
									currentServices: currentListenerEntry.services,
									currentProtocol: currentListenerEntry.servicePort.Protocol,
									newHostname:     service.Hostname,
									newProtocol:     servicePort.Protocol,
								}.addMetric(push)
								break compareWithExisting
							} else {
								continue
							}
						}
						if incomingFilterChain.FilterChainMatch == nil {
							continue
						}

						// We have two non-catch all filter chains. Check for duplicates
						if reflect.DeepEqual(*existingFilterChain.FilterChainMatch, *incomingFilterChain.FilterChainMatch) {
							conflictFound = true
							outboundListenerConflict{
								metric:          model.ProxyStatusConflictOutboundListenerTCPOverTCP,
								node:            node,
								listenerName:    listenerMapKey,
								currentServices: currentListenerEntry.services,
								currentProtocol: currentListenerEntry.servicePort.Protocol,
								newHostname:     service.Hostname,
								newProtocol:     servicePort.Protocol,
							}.addMetric(push)
							break compareWithExisting
						}
					}

					if !conflictFound {
						// There is no conflict with any filter chain in the existing listener.
						// So append the new filter chains to the existing listener's filter chains
						newFilterChains = append(newFilterChains, incomingFilterChain)
						lEntry := listenerMap[listenerMapKey]
						lEntry.services = append(lEntry.services, service)
					}
				}
				currentListenerEntry.listener.FilterChains = newFilterChains
			} else {
				listenerMap[listenerMapKey] = &listenerEntry{
					services:    []*model.Service{service},
					servicePort: servicePort,
					listener:    mutable.Listener,
				}
			}

			if log.DebugEnabled() && len(mutable.Listener.FilterChains) > 1 || currentListenerEntry != nil {
				var numChains int
				if currentListenerEntry != nil {
					numChains = len(currentListenerEntry.listener.FilterChains)
				} else {
					numChains = len(mutable.Listener.FilterChains)
				}
				log.Debugf("buildSidecarOutboundListeners: multiple filter chain listener %s with %d chains", mutable.Listener.Name, numChains)
			}
		}
	}

validateListeners:
	for name, l := range listenerMap {
		if err := l.listener.Validate(); err != nil {
			log.Warnf("buildSidecarOutboundListeners: error validating listener %s (type %v): %v", name, l.servicePort.Protocol, err)
			invalidOutboundListeners.Add(1)
			continue
		}
		if l.servicePort.Protocol.IsTCP() {
			tcpListeners = append(tcpListeners, l.listener)
		} else {
			httpListeners = append(httpListeners, l.listener)
		}
	}

	return append(tcpListeners, httpListeners...)
}

// buildSidecarInboundMgmtListeners creates inbound TCP only listeners for the management ports on
// server (inbound). Management port listeners are slightly different from standard Inbound listeners
// in that, they do not have mixer filters nor do they have inbound auth.
// N.B. If a given management port is same as the service instance's endpoint port
// the pod will fail to start in Kubernetes, because the mixer service tries to
// lookup the service associated with the Pod. Since the pod is yet to be started
// and hence not bound to the service), the service lookup fails causing the mixer
// to fail the health check call. This results in a vicious cycle, where kubernetes
// restarts the unhealthy pod after successive failed health checks, and the mixer
// continues to reject the health checks as there is no service associated with
// the pod.
// So, if a user wants to use kubernetes probes with Istio, she should ensure
// that the health check ports are distinct from the service ports.
func buildSidecarInboundMgmtListeners(node *model.Proxy, env *model.Environment, managementPorts model.PortList, managementIP string) []*xdsapi.Listener {
	listeners := make([]*xdsapi.Listener, 0, len(managementPorts))

	if managementIP == "" {
		managementIP = "127.0.0.1"
	}

	// assumes that inbound connections/requests are sent to the endpoint address
	for _, mPort := range managementPorts {
		switch mPort.Protocol {
		case model.ProtocolHTTP, model.ProtocolHTTP2, model.ProtocolGRPC, model.ProtocolGRPCWeb, model.ProtocolTCP,
			model.ProtocolHTTPS, model.ProtocolTLS, model.ProtocolMongo, model.ProtocolRedis:

			instance := &model.ServiceInstance{
				Endpoint: model.NetworkEndpoint{
					Address:     managementIP,
					Port:        mPort.Port,
					ServicePort: mPort,
				},
				Service: &model.Service{
					Hostname: ManagementClusterHostname,
				},
			}
			listenerOpts := buildListenerOpts{
				ip:   managementIP,
				port: mPort.Port,
				filterChainOpts: []*filterChainOpts{{
					networkFilters: buildInboundNetworkFilters(env, node, instance),
				}},
				// No user filters for the management unless we introduce new listener matches
				skipUserFilters: true,
			}
			l := buildListener(listenerOpts)
			mutable := &plugin.MutableObjects{
				Listener:     l,
				FilterChains: []plugin.FilterChain{{}},
			}
			pluginParams := &plugin.InputParams{
				ListenerProtocol: plugin.ListenerProtocolTCP,
				ListenerCategory: networking.EnvoyFilter_ListenerMatch_SIDECAR_OUTBOUND,
				Env:              env,
				Node:             node,
				Port:             mPort,
			}
			// TODO: should we call plugins for the admin port listeners too? We do everywhere else we construct listeners.
			if err := buildCompleteFilterChain(pluginParams, mutable, listenerOpts); err != nil {
				log.Warna("buildSidecarInboundMgmtListeners ", err.Error())
			} else {
				listeners = append(listeners, l)
			}
		default:
			log.Warnf("Unsupported inbound protocol %v for management port %#v",
				mPort.Protocol, mPort)
		}
	}

	return listeners
}

// httpListenerOpts are options for an HTTP listener
type httpListenerOpts struct {
	//nolint: maligned
	routeConfig      *xdsapi.RouteConfiguration
	rds              string
	useRemoteAddress bool
	direction        http_conn.HttpConnectionManager_Tracing_OperationName
	// If set, use this as a basis
	connectionManager *http_conn.HttpConnectionManager
	// stat prefix for the http connection manager
	// DO not set this field. Will be overridden by buildCompleteFilterChain
	statPrefix string
	// addGRPCWebFilter specifies whether the envoy.grpc_web HTTP filter
	// should be added.
	addGRPCWebFilter bool
}

// filterChainOpts describes a filter chain: a set of filters with the same TLS context
type filterChainOpts struct {
	sniHosts         []string
	destinationCIDRs []string
	tlsContext       *auth.DownstreamTlsContext
	httpOpts         *httpListenerOpts
	match            *listener.FilterChainMatch
	listenerFilters  []listener.ListenerFilter
	networkFilters   []listener.Filter
}

// buildListenerOpts are the options required to build a Listener
type buildListenerOpts struct {
	// nolint: maligned
	env             *model.Environment
	proxy           *model.Proxy
	proxyInstances  []*model.ServiceInstance
	ip              string
	port            int
	bindToPort      bool
	filterChainOpts []*filterChainOpts
	skipUserFilters bool
}

func buildHTTPConnectionManager(node *model.Proxy, env *model.Environment, httpOpts *httpListenerOpts,
	httpFilters []*http_conn.HttpFilter) *http_conn.HttpConnectionManager {

	filters := make([]*http_conn.HttpFilter, len(httpFilters))
	copy(filters, httpFilters)

	if httpOpts.addGRPCWebFilter {
		filters = append(filters, &http_conn.HttpFilter{Name: xdsutil.GRPCWeb})
	}

	filters = append(filters,
		&http_conn.HttpFilter{Name: xdsutil.CORS},
		&http_conn.HttpFilter{Name: xdsutil.Fault},
		&http_conn.HttpFilter{Name: xdsutil.Router},
	)

	if httpOpts.connectionManager == nil {
		httpOpts.connectionManager = &http_conn.HttpConnectionManager{}
	}

	connectionManager := httpOpts.connectionManager
	connectionManager.CodecType = http_conn.AUTO
	connectionManager.AccessLog = []*accesslog.AccessLog{}
	connectionManager.HttpFilters = filters
	connectionManager.StatPrefix = httpOpts.statPrefix
	if httpOpts.useRemoteAddress {
		connectionManager.UseRemoteAddress = proto.BoolTrue
	} else {
		connectionManager.UseRemoteAddress = proto.BoolFalse
	}

	// Allow websocket upgrades
	websocketUpgrade := &http_conn.HttpConnectionManager_UpgradeConfig{UpgradeType: "websocket"}
	connectionManager.UpgradeConfigs = []*http_conn.HttpConnectionManager_UpgradeConfig{websocketUpgrade}
	notimeout := 0 * time.Second
	// Setting IdleTimeout to 0 seems to break most tests, causing
	// envoy to disconnect.
	// connectionManager.IdleTimeout = &notimeout
	connectionManager.StreamIdleTimeout = &notimeout

	if httpOpts.rds != "" {
		rds := &http_conn.HttpConnectionManager_Rds{
			Rds: &http_conn.Rds{
				ConfigSource: core.ConfigSource{
					ConfigSourceSpecifier: &core.ConfigSource_Ads{
						Ads: &core.AggregatedConfigSource{},
					},
				},
				RouteConfigName: httpOpts.rds,
			},
		}
		connectionManager.RouteSpecifier = rds
	} else {
		connectionManager.RouteSpecifier = &http_conn.HttpConnectionManager_RouteConfig{RouteConfig: httpOpts.routeConfig}
	}

	if env.Mesh.AccessLogFile != "" {
		fl := &fileaccesslog.FileAccessLog{
			Path: env.Mesh.AccessLogFile,
		}

		if util.Is11Proxy(node) {
			buildAccessLog(fl, env)
		}

		connectionManager.AccessLog = []*accesslog.AccessLog{
			{
				ConfigType: &accesslog.AccessLog_Config{Config: util.MessageToStruct(fl)},
				Name:       xdsutil.FileAccessLog,
			},
		}
	}

	if env.Mesh.EnableTracing {
		tc := model.GetTraceConfig()
		connectionManager.Tracing = &http_conn.HttpConnectionManager_Tracing{
			OperationName: httpOpts.direction,
			ClientSampling: &envoy_type.Percent{
				Value: tc.ClientSampling,
			},
			RandomSampling: &envoy_type.Percent{
				Value: tc.RandomSampling,
			},
			OverallSampling: &envoy_type.Percent{
				Value: tc.OverallSampling,
			},
		}
		connectionManager.GenerateRequestId = proto.BoolTrue
	}

	return connectionManager
}

// buildListener builds and initializes a Listener proto based on the provided opts. It does not set any filters.
func buildListener(opts buildListenerOpts) *xdsapi.Listener {
	filterChains := make([]listener.FilterChain, 0, len(opts.filterChainOpts))
	listenerFiltersMap := make(map[string]bool)
	var listenerFilters []listener.ListenerFilter

	// add a TLS inspector if we need to detect ServerName or ALPN
	needTLSInspector := false
	for _, chain := range opts.filterChainOpts {
		needsALPN := chain.tlsContext != nil && chain.tlsContext.CommonTlsContext != nil && len(chain.tlsContext.CommonTlsContext.AlpnProtocols) > 0
		if len(chain.sniHosts) > 0 || needsALPN {
			needTLSInspector = true
			break
		}
	}
	if needTLSInspector {
		listenerFiltersMap[envoyListenerTLSInspector] = true
		listenerFilters = append(listenerFilters, listener.ListenerFilter{Name: envoyListenerTLSInspector})
	}

	for _, chain := range opts.filterChainOpts {
		for _, filter := range chain.listenerFilters {
			if _, exist := listenerFiltersMap[filter.Name]; !exist {
				listenerFiltersMap[filter.Name] = true
				listenerFilters = append(listenerFilters, filter)
			}
		}
		match := &listener.FilterChainMatch{}
		needMatch := false
		if chain.match != nil {
			needMatch = true
			match = chain.match
		}
		if len(chain.sniHosts) > 0 {
			sort.Strings(chain.sniHosts)
			fullWildcardFound := false
			for _, h := range chain.sniHosts {
				if h == "*" {
					fullWildcardFound = true
					// If we have a host with *, it effectively means match anything, i.e.
					// no SNI based matching for this host.
					break
				}
			}
			if !fullWildcardFound {
				match.ServerNames = chain.sniHosts
			}
		}
		if len(chain.destinationCIDRs) > 0 {
			sort.Strings(chain.destinationCIDRs)
			for _, d := range chain.destinationCIDRs {
				if len(d) == 0 {
					continue
				}
				cidr := util.ConvertAddressToCidr(d)
				if cidr != nil && cidr.AddressPrefix != model.UnspecifiedIP {
					match.PrefixRanges = append(match.PrefixRanges, cidr)
				}
			}
		}

		if !needMatch && reflect.DeepEqual(*match, listener.FilterChainMatch{}) {
			match = nil
		}
		filterChains = append(filterChains, listener.FilterChain{
			FilterChainMatch: match,
			TlsContext:       chain.tlsContext,
		})
	}

	var deprecatedV1 *xdsapi.Listener_DeprecatedV1
	if !opts.bindToPort {
		deprecatedV1 = &xdsapi.Listener_DeprecatedV1{
			BindToPort: proto.BoolFalse,
		}
	}

	return &xdsapi.Listener{
		Name:            fmt.Sprintf("%s_%d", opts.ip, opts.port),
		Address:         util.BuildAddress(opts.ip, uint32(opts.port)),
		ListenerFilters: listenerFilters,
		FilterChains:    filterChains,
		DeprecatedV1:    deprecatedV1,
	}
}

// buildCompleteFilterChain adds the provided TCP and HTTP filters to the provided Listener and serializes them.
//
// TODO: should we change this from []plugins.FilterChains to [][]listener.Filter, [][]*http_conn.HttpFilter?
// TODO: given how tightly tied listener.FilterChains, opts.filterChainOpts, and mutable.FilterChains are to eachother
// we should encapsulate them some way to ensure they remain consistent (mainly that in each an index refers to the same
// chain)
func buildCompleteFilterChain(pluginParams *plugin.InputParams, mutable *plugin.MutableObjects, opts buildListenerOpts) error {
	if len(opts.filterChainOpts) == 0 {
		return fmt.Errorf("must have more than 0 chains in listener: %#v", mutable.Listener)
	}

	httpConnectionManagers := make([]*http_conn.HttpConnectionManager, len(mutable.FilterChains))
	for i, chain := range mutable.FilterChains {
		opt := opts.filterChainOpts[i]

		if len(chain.TCP) > 0 {
			mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, chain.TCP...)
		}

		if len(opt.networkFilters) > 0 {
			mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, opt.networkFilters...)
		}

		log.Debugf("attached %d network filters to listener %q filter chain %d", len(chain.TCP)+len(opt.networkFilters), mutable.Listener.Name, i)

		if opt.httpOpts != nil {
			opt.httpOpts.statPrefix = mutable.Listener.Name
			httpConnectionManagers[i] = buildHTTPConnectionManager(pluginParams.Node, opts.env, opt.httpOpts, chain.HTTP)
			mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, listener.Filter{
				Name:       xdsutil.HTTPConnectionManager,
				ConfigType: &listener.Filter_Config{Config: util.MessageToStruct(httpConnectionManagers[i])},
			})
			log.Debugf("attached HTTP filter with %d http_filter options to listener %q filter chain %d",
				len(httpConnectionManagers[i].HttpFilters), mutable.Listener.Name, i)
		}
	}

	if !opts.skipUserFilters {
		// NOTE: we have constructed the HTTP connection manager filter above and we are passing the whole filter chain
		// EnvoyFilter crd could choose to replace the HTTP ConnectionManager that we built or can choose to add
		// more filters to the HTTP filter chain. In the latter case, the insertUserFilters function will
		// overwrite the HTTP connection manager in the filter chain after inserting the new filters
		insertUserFilters(pluginParams, mutable.Listener, httpConnectionManagers)
	}

	return nil
}
