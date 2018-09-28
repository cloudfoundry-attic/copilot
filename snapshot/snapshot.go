package snapshot

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/types"

	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	snap "istio.io/istio/pkg/mcp/snapshot"
)

const (
	// TODO: Remove unsupported typeURLs (everything except Gateway, VirtualService, DestinationRule)
	// when mcp client is capable of only sending supported ones
	DestinationRuleTypeURL    = "type.googleapis.com/istio.networking.v1alpha3.DestinationRule"
	VirtualServiceTypeURL     = "type.googleapis.com/istio.networking.v1alpha3.VirtualService"
	GatewayTypeURL            = "type.googleapis.com/istio.networking.v1alpha3.Gateway"
	ServiceEntryTypeURL       = "type.googleapis.com/istio.networking.v1alpha3.ServiceEntry"
	EnvoyFilterTypeURL        = "type.googleapis.com/istio.networking.v1alpha3.EnvoyFilter"
	HTTPAPISpecTypeURL        = "type.googleapis.com/istio.mixer.v1.config.client.HTTPAPISpec"
	HTTPAPISpecBindingTypeURL = "type.googleapis.com/istio.mixer.v1.config.client.HTTPAPISpecBinding"
	QuotaSpecTypeURL          = "type.googleapis.com/istio.mixer.v1.config.client.QuotaSpec"
	QuotaSpecBindingTypeURL   = "type.googleapis.com/istio.mixer.v1.config.client.QuotaSpecBinding"
	PolicyTypeURL             = "type.googleapis.com/istio.authentication.v1alpha1.Policy"
	MeshPolicyTypeURL         = "type.googleapis.com/istio.authentication.v1alpha1.Policy"
	ServiceRoleTypeURL        = "type.googleapis.com/istio.rbac.v1alpha1.ServiceRole"
	ServiceRoleBindingTypeURL = "type.googleapis.com/istio.rbac.v1alpha1.ServiceRoleBinding"
	RbacConfigTypeURL         = "type.googleapis.com/istio.rbac.v1alpha1.RbacConfig"
	defaultGatewayName        = "cloudfoundry-ingress"
	// TODO: Do not specify the nodeID yet as it's used as a key for cache lookup
	// in snapshot, we should add this once the nodeID is configurable in pilot
	node        = ""
	gatewayPort = 80
	servicePort = 8080
)

//go:generate counterfeiter -o fakes/collector.go --fake-name Collector . collector
type collector interface {
	Collect() []*api.RouteWithBackends
}

//go:generate counterfeiter -o fakes/setter.go --fake-name Setter . setter
type setter interface {
	SetSnapshot(node string, istio snap.Snapshot)
}

type Snapshot struct {
	logger       lager.Logger
	ticker       <-chan time.Time
	collector    collector
	setter       setter
	inMemoryShot *snap.InMemory
}

func New(logger lager.Logger, ticker <-chan time.Time, collector collector, setter setter) *Snapshot {
	return &Snapshot{
		logger:    logger,
		ticker:    ticker,
		collector: collector,
		setter:    setter,
	}
}

func (s *Snapshot) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	for {
		select {
		case <-signals:
			return nil
		case <-s.ticker:
			routes := s.collector.Collect()

			gateways := s.createGateways(routes)
			virtualServices := s.createVirtualServices(routes)
			destinationRules := s.createDestinationRules(routes)
			serviceEntries := s.createServiceEntries(routes)

			builder := snap.NewInMemoryBuilder()
			builder.Set(GatewayTypeURL, "1", gateways)

			if s.inMemoryShot == nil {
				builder.Set(VirtualServiceTypeURL, "1", virtualServices)
				builder.Set(DestinationRuleTypeURL, "1", destinationRules)
				builder.Set(ServiceEntryTypeURL, "1", serviceEntries)

				shot := builder.Build()
				s.inMemoryShot = shot
				s.setter.SetSnapshot(node, shot)
				continue
			}

			s.updateBuilder(VirtualServiceTypeURL, virtualServices, builder)
			s.updateBuilder(DestinationRuleTypeURL, destinationRules, builder)
			s.updateBuilder(ServiceEntryTypeURL, serviceEntries, builder)
			shot := builder.Build()
			s.inMemoryShot = shot
			s.setter.SetSnapshot(node, shot)
		}
	}
}

func (s *Snapshot) updateBuilder(typeURL string, envelope []*mcp.Envelope, builder *snap.InMemoryBuilder) {
	version := s.inMemoryShot.Version(typeURL)
	resources := s.inMemoryShot.Resources(typeURL)
	if !reflect.DeepEqual(resources, envelope) {
		builder.Set(typeURL, s.increment(version), envelope)
	}
}

func (s *Snapshot) createGateways(routes []*api.RouteWithBackends) (gaEnvelopes []*mcp.Envelope) {
	gateway := &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   gatewayPort,
					Protocol: "http",
					Name:     "http",
				},
				Hosts: []string{"*"},
			},
		},
	}

	gaResource, err := types.MarshalAny(gateway)
	if err != nil {
		s.logger.Error("envelope.gateway.marshal", err)
	}

	gaEnvelopes = []*mcp.Envelope{
		{
			Metadata: &mcp.Metadata{
				Name:    defaultGatewayName,
				Version: s.version(GatewayTypeURL),
			},
			Resource: gaResource,
		},
	}
	return gaEnvelopes
}

func (s *Snapshot) createDestinationRules(routes []*api.RouteWithBackends) (drEnvelopes []*mcp.Envelope) {
	destinationRules := make(map[string]*model.Config, len(routes))

	for _, route := range routes {
		destinationRuleName := fmt.Sprintf("copilot-rule-for-%s", route.GetHostname())

		var dr *networking.DestinationRule
		if config, ok := destinationRules[destinationRuleName]; ok {
			dr = config.Spec.(*networking.DestinationRule)
		} else {
			dr = createDestinationRule(route)
		}
		dr.Subsets = append(dr.Subsets, createSubset(route.GetCapiProcessGuid()))

		destinationRules[destinationRuleName] = &model.Config{
			ConfigMeta: model.ConfigMeta{
				Type:    model.DestinationRule.Type,
				Version: model.DestinationRule.Version,
				Name:    destinationRuleName,
			},
			Spec: dr,
		}
	}

	for destinationRuleName, dr := range destinationRules {
		drResource, err := types.MarshalAny(dr.Spec)
		if err != nil {
			s.logger.Error("envelope.destinationrule.marshal", err)
		}

		drEnvelopes = append(drEnvelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name:    destinationRuleName,
				Version: s.version(DestinationRuleTypeURL),
			},
			Resource: drResource,
		})
	}

	return drEnvelopes
}

func (s *Snapshot) createVirtualServices(routes []*api.RouteWithBackends) (vsEnvelopes []*mcp.Envelope) {
	virtualServices := make(map[string]*model.Config, len(routes))
	httpRoutes := make(map[string]*networking.HTTPRoute)

	for _, route := range routes {
		virtualServiceName := fmt.Sprintf("copilot-service-for-%s", route.GetHostname())

		var vs *networking.VirtualService
		if config, ok := virtualServices[virtualServiceName]; ok {
			vs = config.Spec.(*networking.VirtualService)
		} else {
			vs = createVirtualService(route)
		}

		if r, ok := httpRoutes[route.GetHostname()+route.GetPath()]; ok {
			r.Route = append(r.Route, createDestinationWeight(route))
		} else {
			r := createHTTPRoute(route)
			if route.GetPath() != "" {
				r.Match = createHTTPMatchRequest(route)
				vs.Http = append([]*networking.HTTPRoute{r}, vs.Http...)
			} else {
				vs.Http = append(vs.Http, r)
			}
			httpRoutes[route.GetHostname()+route.GetPath()] = r
		}

		virtualServices[virtualServiceName] = &model.Config{
			ConfigMeta: model.ConfigMeta{
				Name: virtualServiceName,
			},
			Spec: vs,
		}
	}

	for virtualServiceName, vs := range virtualServices {
		vsResource, err := types.MarshalAny(vs.Spec)
		if err != nil {
			s.logger.Error("envelope.virtualservice.marshal", err)
		}

		vsEnvelopes = append(vsEnvelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name:    virtualServiceName,
				Version: s.version(VirtualServiceTypeURL),
			},
			Resource: vsResource,
		})
	}

	return vsEnvelopes
}

func (s *Snapshot) createServiceEntries(routes []*api.RouteWithBackends) (seEnvelopes []*mcp.Envelope) {
	serviceEntries := make(map[string]*model.Config, len(routes))

	for _, route := range routes {
		serviceEntryName := fmt.Sprintf("copilot-service-entry-for-%s", route.GetHostname())

		if route.Backends.GetBackends() != nil || len(route.Backends.GetBackends()) != 0 {
			var se *networking.ServiceEntry
			if config, ok := serviceEntries[serviceEntryName]; ok {
				se = config.Spec.(*networking.ServiceEntry)
			} else {
				se = createServiceEntry(route)
			}
			se.Endpoints = append(se.Endpoints, createEndpoint(route.Backends.GetBackends(), route.GetCapiProcessGuid())...)

			serviceEntries[serviceEntryName] = &model.Config{
				ConfigMeta: model.ConfigMeta{
					Name: serviceEntryName,
				},
				Spec: se,
			}
		}
	}

	for serviceEntryName, se := range serviceEntries {
		seResource, err := types.MarshalAny(se.Spec)
		if err != nil {
			s.logger.Error("envelope.serviceentry.marshal", err)
		}

		seEnvelopes = append(seEnvelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name:    serviceEntryName,
				Version: s.version(ServiceEntryTypeURL),
			},
			Resource: seResource,
		})
	}

	return seEnvelopes
}

func (s *Snapshot) version(typeURL string) string {
	if s.inMemoryShot == nil {
		return "1"
	}
	return s.inMemoryShot.Version(typeURL)
}

func (s *Snapshot) increment(version string) string {
	v, err := strconv.Atoi(version)
	if err != nil {
		s.logger.Error("run.inmemory.version", err)
	}
	v++
	return strconv.Itoa(v)
}

func createEndpoint(backends []*api.Backend, capiProcessGuid string) []*networking.ServiceEntry_Endpoint {
	endpoints := make([]*networking.ServiceEntry_Endpoint, 0)
	for _, backend := range backends {
		endpoints = append(endpoints, &networking.ServiceEntry_Endpoint{
			Address: backend.GetAddress(),
			Ports: map[string]uint32{
				"http": backend.GetPort(),
			},
			Labels: map[string]string{"cfapp": capiProcessGuid},
		})
	}
	return endpoints
}

func createServiceEntry(route *api.RouteWithBackends) *networking.ServiceEntry {
	return &networking.ServiceEntry{
		Hosts: []string{route.GetHostname()},
		Ports: []*networking.Port{
			{
				Name:     "http",
				Number:   servicePort,
				Protocol: "http",
			},
		},
		Location:   networking.ServiceEntry_MESH_INTERNAL,
		Resolution: networking.ServiceEntry_STATIC,
	}
}

func createVirtualService(route *api.RouteWithBackends) *networking.VirtualService {
	return &networking.VirtualService{
		Gateways: []string{defaultGatewayName},
		Hosts:    []string{route.GetHostname()},
	}
}

func createDestinationWeight(route *api.RouteWithBackends) *networking.DestinationWeight {
	return &networking.DestinationWeight{
		Destination: &networking.Destination{
			Host:   route.GetHostname(),
			Subset: route.GetCapiProcessGuid(),
			Port: &networking.PortSelector{
				Port: &networking.PortSelector_Number{
					Number: servicePort,
				},
			},
		},
		Weight: route.GetRouteWeight(),
	}
}

func createHTTPRoute(route *api.RouteWithBackends) *networking.HTTPRoute {
	return &networking.HTTPRoute{
		Route: []*networking.DestinationWeight{createDestinationWeight(route)},
	}
}

func createHTTPMatchRequest(route *api.RouteWithBackends) []*networking.HTTPMatchRequest {
	return []*networking.HTTPMatchRequest{
		{
			Uri: &networking.StringMatch{
				MatchType: &networking.StringMatch_Prefix{
					Prefix: route.GetPath(),
				},
			},
		},
	}
}

func createSubset(capiProcessGUID string) *networking.Subset {
	return &networking.Subset{
		Name:   capiProcessGUID,
		Labels: map[string]string{"cfapp": capiProcessGUID},
	}
}

func createDestinationRule(route *api.RouteWithBackends) *networking.DestinationRule {
	return &networking.DestinationRule{
		Host: route.GetHostname(),
	}
}
