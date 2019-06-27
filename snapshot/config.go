package snapshot

import (
	"code.cloudfoundry.org/silk-release/src/lib/policy_client"
	"fmt"
	"time"

	"code.cloudfoundry.org/copilot/certs"
	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/types"
	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
)

//go:generate counterfeiter -o fakes/config.go --fake-name Config . config
type config interface {
	CreateGatewayResources() []*mcp.Resource
	CreateSidecarResources(routes []*models.RouteWithBackends, policies []policy_client.Policy, version string) []*mcp.Resource
	CreateVirtualServiceResources(routes []*models.RouteWithBackends, version string) []*mcp.Resource
	CreateDestinationRuleResources(routes []*models.RouteWithBackends, version string) []*mcp.Resource
	CreateServiceEntryResources(routes []*models.RouteWithBackends, version string) []*mcp.Resource
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

func (c *Config) CreateSidecarResources(routes []*models.RouteWithBackends, policies []policy_client.Policy, version string) (resources []*mcp.Resource) {
	// match destinations w routes
	routeMap := map[string][]string{}
	for _, route := range routes {
		processGuid := route.CapiProcessGUID
		if _, ok := routeMap[processGuid]; ok {
			routeMap[processGuid] = append(routeMap[processGuid], route.Hostname)
		} else {
			routeMap[processGuid] = []string{route.Hostname}
		}
	}

	// process to get all destinations per source
	policyMap := map[string][]string{}
	for _, policy := range policies {
		source := policy.Source.ID
		destination := policy.Destination.ID
		if _, ok := policyMap[source]; ok {
			policyMap[source] = append(policyMap[source], routeMap[destination]...)
		} else {
			policyMap[source] = routeMap[destination]
		}
		//uniquify down the road this policy map for duplicate hosts
	}

	sidecar := &networking.Sidecar{
		Egress: []*networking.IstioEgressListener{
			&networking.IstioEgressListener{
				Hosts: []string{},
			},
		},
	}
	scResource, err := types.MarshalAny(sidecar)
	if err != nil {
		c.logger.Error("marshaling sidecar", err)
	}

	// create default sidecar config w/ empty hosts
	resources = append(resources, &mcp.Resource{
		Metadata: &mcp.Metadata{
			Name:    "default",
			Version: version,
		},
		Body: scResource,
	})

	for source, destinations := range policyMap {
		for index, _ := range destinations {
			destinations[index] = "internal/" + destinations[index]
		}
		// create sidecar resources:
		// apiVersion: networking.istio.io/v1alpha3
		// kind: Sidecar
		// metadata:
		// 	name: source guid
		// spec:
		// 	egress:
		// 	- hosts:
		// 			- internal/<destination routes>
		// 	workloadSelector:
		// 		labels:
		// 			cfapp: source guid
		sidecar = &networking.Sidecar{
			WorkloadSelector: &networking.WorkloadSelector{
				Labels: map[string]string{
					"cfapp": source,
				},
			},
			Egress: []*networking.IstioEgressListener{
				&networking.IstioEgressListener{
					Hosts: destinations,
				},
			},
		}

		scResource, err = types.MarshalAny(sidecar)
		if err != nil {
			c.logger.Error("marshaling sidecar", err)
		}
		resources = append(resources, &mcp.Resource{
			Metadata: &mcp.Metadata{
				Name:    source,
				Version: version,
			},
			Body: scResource,
		})
	}

	return resources
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
