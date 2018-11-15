package snapshot

import (
	"fmt"

	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/types"
	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
)

//go:generate counterfeiter -o fakes/config.go --fake-name Config . config
type config interface {
	CreateGatewayEnvelopes() []*mcp.Envelope
	CreateVirtualServiceEnvelopes(routes []*models.RouteWithBackends, version string) []*mcp.Envelope
	CreateDestinationRuleEnvelopes(routes []*models.RouteWithBackends, version string) []*mcp.Envelope
	CreateServiceEntryEnvelopes(routes []*models.RouteWithBackends, version string) []*mcp.Envelope
}

type Config struct {
	logger lager.Logger
}

func NewConfig(logger lager.Logger) *Config {
	return &Config{logger: logger}
}

func (c *Config) CreateGatewayEnvelopes() (envelopes []*mcp.Envelope) {
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
		c.logger.Error("marshaling gateway", err)
	}

	envelopes = []*mcp.Envelope{
		{
			Metadata: &mcp.Metadata{
				Name:    DefaultGatewayName,
				Version: "1",
			},
			Resource: gaResource,
		},
	}

	return envelopes
}

func (c *Config) CreateDestinationRuleEnvelopes(routes []*models.RouteWithBackends, version string) (envelopes []*mcp.Envelope) {
	destinationRules := make(map[string]*model.Config, len(routes))

	for _, route := range routes {
		if route.Internal {
			continue
		}

		var destinationRule *networking.DestinationRule
		destinationRuleName := fmt.Sprintf("copilot-rule-for-%s", route.Hostname)

		if config, ok := destinationRules[destinationRuleName]; ok {
			destinationRule = config.Spec.(*networking.DestinationRule)
		} else {
			destinationRule = &networking.DestinationRule{Host: route.Hostname}
		}

		subset := &networking.Subset{Name: route.CapiProcessGUID,
			Labels: map[string]string{"cfapp": route.CapiProcessGUID},
		}
		destinationRule.Subsets = append(destinationRule.Subsets, subset)

		destinationRules[destinationRuleName] = &model.Config{
			ConfigMeta: model.ConfigMeta{
				Type:    model.DestinationRule.Type,
				Version: model.DestinationRule.Version,
				Name:    destinationRuleName,
			},
			Spec: destinationRule,
		}
	}

	for destinationRuleName, dr := range destinationRules {
		drResource, err := types.MarshalAny(dr.Spec)
		if err != nil {
			c.logger.Error("marshaling destination rule", err)
		}

		envelopes = append(envelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name:    destinationRuleName,
				Version: version,
			},
			Resource: drResource,
		})
	}

	return envelopes
}

func (c *Config) CreateVirtualServiceEnvelopes(routes []*models.RouteWithBackends, version string) (envelopes []*mcp.Envelope) {
	virtualServices := make(map[string]*model.Config, len(routes))
	httpRoutes := make(map[string]*networking.HTTPRoute)

	for _, route := range routes {
		var vs *networking.VirtualService
		virtualServiceName := fmt.Sprintf("copilot-service-for-%s", route.Hostname)

		if config, ok := virtualServices[virtualServiceName]; ok {
			vs = config.Spec.(*networking.VirtualService)
		} else {
			vs = &networking.VirtualService{Hosts: []string{route.Hostname}}

			if !route.Internal {
				vs.Gateways = []string{DefaultGatewayName}
			}
		}

		fullRoute := route.Hostname + route.Path
		if r, ok := httpRoutes[fullRoute]; ok {
			r.Route = append(r.Route, createDestinationWeight(route))
		} else {
			r := &networking.HTTPRoute{
				Route: []*networking.HTTPRouteDestination{createDestinationWeight(route)},
			}

			if route.Path != "" {
				if !route.Internal {
					r.Match = []*networking.HTTPMatchRequest{
						{
							Uri: &networking.StringMatch{
								MatchType: &networking.StringMatch_Prefix{
									Prefix: route.Path,
								},
							},
						},
					}
				}

				vs.Http = append([]*networking.HTTPRoute{r}, vs.Http...)
			} else {
				vs.Http = append(vs.Http, r)
			}
			httpRoutes[fullRoute] = r
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
			c.logger.Error("marshaling virtual service", err)
		}

		envelopes = append(envelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name:    virtualServiceName,
				Version: version,
			},
			Resource: vsResource,
		})
	}

	return envelopes
}

func (c *Config) CreateServiceEntryEnvelopes(routes []*models.RouteWithBackends, version string) (envelopes []*mcp.Envelope) {
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
			c.logger.Error("marshaling service entry", err)
		}

		envelopes = append(envelopes, &mcp.Envelope{
			Metadata: &mcp.Metadata{
				Name:    serviceEntryName,
				Version: version,
			},
			Resource: seResource,
		})
	}

	return envelopes
}

func createEndpoint(route *models.RouteWithBackends) []*networking.ServiceEntry_Endpoint {
	endpoints := make([]*networking.ServiceEntry_Endpoint, 0)
	portType := "http"

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
