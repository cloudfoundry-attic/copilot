package snapshot

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/copilot/certs"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/types"
	authentication "istio.io/api/authentication/v1alpha1"
	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
)

//go:generate counterfeiter -o fakes/config.go --fake-name Config . config
type config interface {
	CreateGatewayResources() []*mcp.Resource
	CreateSidecarResources() []*mcp.Resource
	CreateVirtualServiceResources(routes []*models.RouteWithBackends, version string) []*mcp.Resource
	CreateDestinationRuleResources(routes []*models.RouteWithBackends, version string) []*mcp.Resource
	CreateServiceEntryResources(routes []*models.RouteWithBackends, version string) []*mcp.Resource
	CreatePolicyResources() []*mcp.Resource
}

type Config struct {
	logger    lager.Logger
	librarian certs.Librarian
}

func NewConfig(librarian certs.Librarian, logger lager.Logger) *Config {
	return &Config{
		librarian: librarian,
		logger:    logger,
	}
}

func (c *Config) CreateSidecarResources() []*mcp.Resource {
	sidecar := &networking.Sidecar{
		Egress: []*networking.IstioEgressListener{
			&networking.IstioEgressListener{
				Hosts: []string{
					"internal/*",
				},
			},
		},
	}

	scResource, err := types.MarshalAny(sidecar)
	if err != nil {
		// not tested
		c.logger.Error("marshaling gateway", err)
	}

	return []*mcp.Resource{
		&mcp.Resource{
			Metadata: &mcp.Metadata{
				Name:    DefaultSidecarName,
				Version: "1",
			},
			Body: scResource,
		},
	}
}

func (c *Config) CreateGatewayResources() (resources []*mcp.Resource) {
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

	certPairs, err := c.librarian.Locate()
	if err != nil {
		c.logger.Error("create gateway", err)
	}

	if len(certPairs) > 0 {
		for _, certPair := range certPairs {
			gateway.Servers = append(gateway.Servers, &networking.Server{
				Port: &networking.Port{
					Number:   443,
					Protocol: "https",
					Name:     certPair.Hosts[0],
				},
				Tls: &networking.Server_TLSOptions{
					Mode:              networking.Server_TLSOptions_SIMPLE,
					ServerCertificate: certPair.CertPath,
					PrivateKey:        certPair.KeyPath,
				},
				Hosts: certPair.Hosts,
			})
		}
	}

	gaResource, err := types.MarshalAny(gateway)
	if err != nil {
		c.logger.Error("marshaling gateway", err)
	}

	resources = []*mcp.Resource{
		{
			Metadata: &mcp.Metadata{
				Name:    DefaultGatewayName,
				Version: "1",
			},
			Body: gaResource,
		},
	}

	return resources
}

func (c *Config) CreatePolicyResources() []*mcp.Resource {
	policy := &authentication.Policy{
		Peers: []*authentication.PeerAuthenticationMethod{
			{
				Params: &authentication.PeerAuthenticationMethod_Mtls{
					Mtls: &authentication.MutualTls{
						Mode: authentication.MutualTls_STRICT,
					},
				},
			},
		},
	}

	policyResource, err := types.MarshalAny(policy)
	if err != nil {
		// not tested
		c.logger.Error("marshaling policy", err)
	}

	return []*mcp.Resource{
		&mcp.Resource{
			Metadata: &mcp.Metadata{
				Name:    "default",
				Version: "1",
			},
			Body: policyResource,
		},
	}
}

func (c *Config) CreateDestinationRuleResources(routes []*models.RouteWithBackends, version string) (resources []*mcp.Resource) {
	destinationRules := make(map[string]*model.Config, len(routes))

	for _, route := range routes {
		var destinationRule *networking.DestinationRule
		destinationRuleName := fmt.Sprintf("copilot-rule-for-%s", route.Hostname)

		if route.Internal {
			destinationRuleName = fmt.Sprintf("internal/copilot-rule-for-%s", route.Hostname)
		}

		if config, ok := destinationRules[destinationRuleName]; ok {
			destinationRule = config.Spec.(*networking.DestinationRule)
		} else {
			destinationRule = &networking.DestinationRule{Host: route.Hostname}
		}

		if route.Internal {
			trafficPolicy := &networking.TrafficPolicy{
				Tls: &networking.TLSSettings{
					Mode:              2,
					ClientCertificate: "/etc/cf-instance-credentials/instance.crt",
					PrivateKey:        "/etc/cf-instance-credentials/instance.key",
					CaCertificates:    "/etc/cf-system-certificates/trusted-ca-1.crt",
				},
			}

			destinationRule.TrafficPolicy = trafficPolicy
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

		resources = append(resources, &mcp.Resource{
			Metadata: &mcp.Metadata{
				Name:    destinationRuleName,
				Version: version,
			},
			Body: drResource,
		})
	}

	return resources
}

func (c *Config) CreateVirtualServiceResources(routes []*models.RouteWithBackends, version string) (resources []*mcp.Resource) {
	virtualServices := make(map[string]*model.Config, len(routes))
	httpRoutes := make(map[string]*networking.HTTPRoute)

	for _, route := range routes {
		var vs *networking.VirtualService
		virtualServiceName := fmt.Sprintf("copilot-service-for-%s", route.Hostname)

		if route.Internal {
			virtualServiceName = fmt.Sprintf("internal/copilot-service-for-%s", route.Hostname)
		}

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
			r := createHTTPRoute(route)
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

		resources = append(resources, &mcp.Resource{
			Metadata: &mcp.Metadata{
				Name:    virtualServiceName,
				Version: version,
			},
			Body: vsResource,
		})
	}

	return resources
}

func (c *Config) CreateServiceEntryResources(routes []*models.RouteWithBackends, version string) (resources []*mcp.Resource) {
	serviceEntries := make(map[string]*model.Config, len(routes))

	for _, route := range routes {
		serviceEntryName := fmt.Sprintf("copilot-service-entry-for-%s", route.Hostname)

		if route.Internal {
			serviceEntryName = fmt.Sprintf("internal/copilot-service-entry-for-%s", route.Hostname)
		}

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

		resources = append(resources, &mcp.Resource{
			Metadata: &mcp.Metadata{
				Name:    serviceEntryName,
				Version: version,
			},
			Body: seResource,
		})
	}

	return resources
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

	uniqueContainerPorts := map[uint32]struct{}{}
	for _, backend := range route.Backends.Backends {
		uniqueContainerPorts[backend.ContainerPort] = struct{}{}
	}

	var serviceEntryPorts []*networking.Port
	for containerPort := range uniqueContainerPorts {
		serviceEntryPorts = append(serviceEntryPorts, &networking.Port{
			Name:     protocol,
			Number:   containerPort,
			Protocol: protocol,
		})
	}

	return &networking.ServiceEntry{
		Hosts:      []string{route.Hostname},
		Addresses:  addresses,
		Ports:      serviceEntryPorts,
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
					Number: route.Backends.Backends[0].ContainerPort,
				},
			},
		},
		Weight: route.RouteWeight,
	}
}

func createHTTPRoute(route *models.RouteWithBackends) *networking.HTTPRoute {
	if route.Internal {
		return &networking.HTTPRoute{
			Route: []*networking.HTTPRouteDestination{createDestinationWeight(route)},
			Retries: &networking.HTTPRetry{
				Attempts: 3,
				RetryOn:  "5xx",
			},
			Timeout: types.DurationProto(15 * time.Second),
		}
	}

	return &networking.HTTPRoute{
		Route: []*networking.HTTPRouteDestination{createDestinationWeight(route)},
	}
}
