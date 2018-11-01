package snapshot

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/types"

	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	snap "istio.io/istio/pkg/mcp/snapshot"
)

const (
	// TODO: Remove unsupported typeURLs (everything except Gateway, VirtualService, DestinationRule)
	// when mcp client is capable of only sending a subset of the types
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
	DefaultGatewayName        = "cloudfoundry-ingress"
	// TODO: Do not specify the nodeID yet as it's used as a key for cache lookup
	// in snapshot, we should add this once the nodeID is configurable in pilot
	node        = "default"
	gatewayPort = 80
	servicePort = 8080
)

//go:generate counterfeiter -o fakes/collector.go --fake-name Collector . collector
type collector interface {
	Collect() []*models.RouteWithBackends
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
	builder      *snap.InMemoryBuilder
	cachedRoutes []*models.RouteWithBackends
	ver          int
}

func New(logger lager.Logger, ticker <-chan time.Time, collector collector, setter setter, builder *snap.InMemoryBuilder) *Snapshot {
	return &Snapshot{
		logger:    logger,
		ticker:    ticker,
		collector: collector,
		setter:    setter,
		builder:   builder,
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

			if reflect.DeepEqual(routes, s.cachedRoutes) {
				continue
			}

			newVersion := s.increment()
			s.cachedRoutes = routes

			gateways := s.createGateways(routes)
			virtualServices := s.createVirtualServices(routes)
			destinationRules := s.createDestinationRules(routes)
			serviceEntries := s.createServiceEntries(routes)

			s.builder.Set(GatewayTypeURL, "1", gateways)

			s.builder.Set(VirtualServiceTypeURL, newVersion, virtualServices)
			s.builder.Set(DestinationRuleTypeURL, newVersion, destinationRules)
			s.builder.Set(ServiceEntryTypeURL, newVersion, serviceEntries)

			shot := s.builder.Build()
			s.setter.SetSnapshot(node, shot)
			s.builder = shot.Builder()
		}
	}
}

func (s *Snapshot) createGateways(routes []*models.RouteWithBackends) (gaEnvelopes []*mcp.Envelope) {
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
				Name:    DefaultGatewayName,
				Version: s.version(),
			},
			Resource: gaResource,
		},
	}
	return gaEnvelopes
}

func (s *Snapshot) createDestinationRules(routes []*models.RouteWithBackends) (drEnvelopes []*mcp.Envelope) {
	destinationRules := make(map[string]*model.Config, len(routes))

	for _, route := range routes {
		if route.Internal {
			continue
		}
		destinationRuleName := fmt.Sprintf("copilot-rule-for-%s", route.Hostname)

		var dr *networking.DestinationRule
		if config, ok := destinationRules[destinationRuleName]; ok {
			dr = config.Spec.(*networking.DestinationRule)
		} else {
			dr = createDestinationRule(route)
		}
		dr.Subsets = append(dr.Subsets, createSubset(route.CapiProcessGUID))

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
				Version: s.version(),
			},
			Resource: drResource,
		})
	}

	return drEnvelopes
}

func (s *Snapshot) createVirtualServices(routes []*models.RouteWithBackends) (vsEnvelopes []*mcp.Envelope) {
	virtualServices := make(map[string]*model.Config, len(routes))
	httpRoutes := make(map[string]*networking.HTTPRoute)

	for _, route := range routes {
		virtualServiceName := fmt.Sprintf("copilot-service-for-%s", route.Hostname)

		var vs *networking.VirtualService
		if config, ok := virtualServices[virtualServiceName]; ok {
			vs = config.Spec.(*networking.VirtualService)
		} else {
			vs = createVirtualService(route)
		}

		if r, ok := httpRoutes[route.Hostname+route.Path]; ok {
			r.Route = append(r.Route, createDestinationWeight(route))
		} else {
			r := createHTTPRoute(route)
			if route.Path != "" {
				r.Match = createHTTPMatchRequest(route)
				vs.Http = append([]*networking.HTTPRoute{r}, vs.Http...)
			} else {
				vs.Http = append(vs.Http, r)
			}
			httpRoutes[route.Hostname+route.Path] = r
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
				Version: s.version(),
			},
			Resource: vsResource,
		})
	}

	return vsEnvelopes
}

func (s *Snapshot) createServiceEntries(routes []*models.RouteWithBackends) (seEnvelopes []*mcp.Envelope) {
	serviceEntries := make(map[string]*model.Config, len(routes))

	for _, route := range routes {
		serviceEntryName := fmt.Sprintf("copilot-service-entry-for-%s", route.Hostname)

		if route.Backends.Backends != nil || len(route.Backends.Backends) != 0 {
			var se *networking.ServiceEntry
			if config, ok := serviceEntries[serviceEntryName]; ok {
				se = config.Spec.(*networking.ServiceEntry)
			} else {
				se = createServiceEntry(route)
			}
			se.Endpoints = append(se.Endpoints, createEndpoint(route)...)

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
				Version: s.version(),
			},
			Resource: seResource,
		})
	}

	return seEnvelopes
}

func (s *Snapshot) version() string {
	return strconv.Itoa(s.ver)
}

func (s *Snapshot) increment() string {
	s.ver++
	return s.version()
}

func createEndpoint(route *models.RouteWithBackends) []*networking.ServiceEntry_Endpoint {
	endpoints := make([]*networking.ServiceEntry_Endpoint, 0)
	portType := "http"
	if route.Internal {
		portType = "tcp"
	}
	for _, backend := range route.Backends.Backends {
		endpoints = append(endpoints, &networking.ServiceEntry_Endpoint{
			Address: backend.Address,
			Ports: map[string]uint32{
				portType: backend.Port,
			},
			Labels: map[string]string{"cfapp": route.CapiProcessGUID},
		})
	}
	return endpoints
}

func createServiceEntry(route *models.RouteWithBackends) *networking.ServiceEntry {
	protocol := "http"
	var addresses []string
	if route.Internal {
		addresses = []string{route.VIP}
		protocol = "tcp"
	}

	return &networking.ServiceEntry{
		Hosts:     []string{route.Hostname},
		Addresses: addresses,
		Ports: []*networking.Port{
			{
				Name:     protocol,
				Number:   servicePort,
				Protocol: protocol,
			},
		},
		Location:   networking.ServiceEntry_MESH_INTERNAL,
		Resolution: networking.ServiceEntry_STATIC,
	}
}

func createVirtualService(route *models.RouteWithBackends) *networking.VirtualService {
	if route.Internal {
		return &networking.VirtualService{
			Hosts: []string{route.Hostname},
		}
	}
	return &networking.VirtualService{
		Gateways: []string{DefaultGatewayName},
		Hosts:    []string{route.Hostname},
	}
}

func createDestinationWeight(route *models.RouteWithBackends) *networking.HTTPRouteDestination {
	return &networking.HTTPRouteDestination{
		Destination: &networking.Destination{
			Host:   route.Hostname,
			Subset: route.CapiProcessGUID,
			Port: &networking.PortSelector{
				Port: &networking.PortSelector_Number{
					Number: servicePort,
				},
			},
		},
		Weight: route.RouteWeight,
	}
}

func createHTTPRoute(route *models.RouteWithBackends) *networking.HTTPRoute {
	return &networking.HTTPRoute{
		Route: []*networking.HTTPRouteDestination{createDestinationWeight(route)},
	}
}

func createHTTPMatchRequest(route *models.RouteWithBackends) []*networking.HTTPMatchRequest {
	if route.Internal {
		return []*networking.HTTPMatchRequest{}
	}
	return []*networking.HTTPMatchRequest{
		{
			Uri: &networking.StringMatch{
				MatchType: &networking.StringMatch_Prefix{
					Prefix: route.Path,
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

func createDestinationRule(route *models.RouteWithBackends) *networking.DestinationRule {
	return &networking.DestinationRule{
		Host: route.Hostname,
	}
}
