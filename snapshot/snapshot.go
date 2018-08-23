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
	VSTypeURL = "type.googleapis.com/istio.networking.v1alpha3.DestinationRule"
	DRTypeURL = "type.googleapis.com/istio.networking.v1alpha3.VirtualService"
	node      = "copilot-node-id"
)

//go:generate counterfeiter -o fakes/collector.go --fake-name Collector . collector
type collector interface {
	Collect() []*api.RouteWithBackends
}

type builder interface {
	Set(typeURL string, version string, resources []*mcp.Envelope)
	Build() *snap.InMemory
}

//go:generate counterfeiter -o fakes/setter.go --fake-name Setter . setter
type setter interface {
	SetSnapshot(node string, istio snap.Snapshot)
}

type Snapshot struct {
	logger    lager.Logger
	ticker    <-chan time.Time
	collector collector
	builder   builder
	setter    setter
}

func New(logger lager.Logger, ticker <-chan time.Time, collector collector, builder builder, setter setter) *Snapshot {
	return &Snapshot{
		logger:    logger,
		ticker:    ticker,
		collector: collector,
		builder:   builder,
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
			vsEnvelopes, drEnvelopes := s.createEnvelopes(routes)

			s.builder.Set(VSTypeURL, "1", vsEnvelopes)
			s.builder.Set(DRTypeURL, "1", drEnvelopes)

			shot := s.builder.Build()
			s.setter.SetSnapshot(node, shot)
		}
	}
}

func (s *Snapshot) createEnvelopes(routes []*api.RouteWithBackends) ([]*mcp.Envelope, []*mcp.Envelope) {
	var (
		vsEnvelopes []*mcp.Envelope
		drEnvelopes []*mcp.Envelope
	)

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

	return vsEnvelopes, drEnvelopes
}

func createVirtualService(route *api.RouteWithBackends) *networking.VirtualService {
	return &networking.VirtualService{
		Gateways: []string{"cloudfoundry-ingress"},
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
