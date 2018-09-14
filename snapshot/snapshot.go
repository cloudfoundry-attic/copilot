package snapshot

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"

	"github.com/gogo/protobuf/types"

	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
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
	node = ""
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
	logger    lager.Logger
	ticker    <-chan time.Time
	collector collector
	setter    setter
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
			builder := snap.NewInMemoryBuilder()
			routes := s.collector.Collect()
			gateways, virtualservices, destinationrules := s.createEnvelopes(routes)

			//TODO send incrementing versions
			builder.Set(GatewayTypeURL, "1", gateways)
			builder.Set(VirtualServiceTypeURL, "1", virtualservices)
			builder.Set(DestinationRuleTypeURL, "1", destinationrules)

			shot := builder.Build()
			s.setter.SetSnapshot(node, shot)
		}
	}
}

func (s *Snapshot) createEnvelopes(routes []*api.RouteWithBackends) (gaEnvelopes, vsEnvelopes, drEnvelopes []*mcp.Envelope) {
	gateway := &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   80,
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
				Name: defaultGatewayName,
			},
			Resource: gaResource,
		},
	}

	httpRoutes := make(map[string]*networking.HTTPRoute)

	for _, route := range routes {
		destinationRuleName := fmt.Sprintf("copilot-rule-for-%s", route.GetHostname())
		virtualServiceName := fmt.Sprintf("copilot-service-for-%s", route.GetHostname())

		dr := createDestinationRule(route)
		dr.Subsets = append(dr.Subsets, createSubset(route.GetCapiProcessGuid()))

		vs := createVirtualService(route)

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

		vsResource, err := types.MarshalAny(vs)
		if err != nil {
			s.logger.Error("envelope.virtualservice.marshal", err) //untested
		}

		vsEnvelopes = append(vsEnvelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name: virtualServiceName,
			},
			Resource: vsResource,
		})

		drResource, err := types.MarshalAny(dr)
		if err != nil {
			s.logger.Error("envelope.destinationrule.marshal", err) //untested
		}

		drEnvelopes = append(drEnvelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name: destinationRuleName,
			},
			Resource: drResource,
		})
	}

	return gaEnvelopes, vsEnvelopes, drEnvelopes
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
					Number: 8080,
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
