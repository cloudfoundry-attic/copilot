package snapshot

import (
	"fmt"
	"os"
	// "reflect"
	"strconv"
	"time"

	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"

	"istio.io/istio/pilot/pkg/model"
	snap "istio.io/istio/pkg/mcp/snapshot"
)

var (
	// TODO: Remove unsupported typeURLs (everything except Gateway, VirtualService, DestinationRule)
	// when mcp client is capable of only sending a subset of the types
	DestinationRuleTypeURL    string
	VirtualServiceTypeURL     string
	GatewayTypeURL            string
	ServiceEntryTypeURL       string
	EnvoyFilterTypeURL        string
	SidecarTypeURL            string
	HTTPAPISpecTypeURL        string
	HTTPAPISpecBindingTypeURL string
	QuotaSpecTypeURL          string
	QuotaSpecBindingTypeURL   string
	PolicyTypeURL             string
	MeshPolicyTypeURL         string
	ServiceRoleTypeURL        string
	ServiceRoleBindingTypeURL string
	RbacConfigTypeURL         string
	ClusterRbacConfigTypeURL  string
)

const (
	DefaultGatewayName = "cloudfoundry-ingress"
	DefaultSidecarName = "cloudfoundry-sidecar"

	// TODO: Do not specify the nodeID yet as it's used as a key for cache lookup
	// in snapshot, we should add this once the nodeID is configurable in pilot
	node        = "default"
	gatewayPort = 80
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
	config       config
	ver          int
}

func New(logger lager.Logger, ticker <-chan time.Time, collector collector, setter setter, builder *snap.InMemoryBuilder, config config) *Snapshot {
	return &Snapshot{
		logger:    logger,
		ticker:    ticker,
		collector: collector,
		setter:    setter,
		builder:   builder,
		config:    config,
	}
}

func (s *Snapshot) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	for {
		fmt.Println("Snapshot Run func")
		select {
		case <-signals:
			fmt.Println("Signal")
			return nil
		case <-s.ticker:
			fmt.Println("Tick")
			routes := s.collector.Collect()

			// if reflect.DeepEqual(routes, s.cachedRoutes) {
			// 	continue
			// }
			fmt.Printf("Routes %+v", routes)
			newVersion := s.increment()
			s.cachedRoutes = routes

			fmt.Println("About to create Resources")
			gateways := s.config.CreateGatewayResources()
			fmt.Printf("gateways: %+v", gateways)
			sidecars := s.config.CreateSidecarResources()
			virtualServices := s.config.CreateVirtualServiceResources(routes, newVersion)
			fmt.Printf("virtual services: %+v", virtualServices)
			destinationRules := s.config.CreateDestinationRuleResources(routes, newVersion)
			fmt.Printf("destination rules: %+v", destinationRules)
			serviceEntries := s.config.CreateServiceEntryResources(routes, newVersion)
			fmt.Printf("service entries: %+v", serviceEntries)
			emptyRbacConfig := s.config.EmptyRBACConfigResources()
			emptyQuotaSpec := s.config.EmptyQuotaSpecResources()
			emptyQuotaSpecBinding := s.config.EmptyQuotaSpecBindingResources()
			emptyHTTPAPISpecBinding := s.config.EmptyHTTPAPISpecBindingResources()
			emptyHTTPAPISpec := s.config.EmptyHTTPAPISpecResources()
			emptyServiceRoleBinding := s.config.EmptyServiceRoleBindingResources()
			emptyServiceRole := s.config.EmptyServiceRoleResources()
			emptyPolicy := s.config.EmptyPolicyResources()
			emptyEnvoyFilter := s.config.EmptyEnvoyFilterResources()

			fmt.Println("About to build the set of resources")
			s.builder.Set(GatewayTypeURL, "1", gateways)
			s.builder.Set(SidecarTypeURL, "1", sidecars)
			s.builder.Set(VirtualServiceTypeURL, newVersion, virtualServices)
			s.builder.Set(DestinationRuleTypeURL, newVersion, destinationRules)
			s.builder.Set(ServiceEntryTypeURL, newVersion, serviceEntries)
			s.builder.Set(RbacConfigTypeURL, "1", emptyRbacConfig)
			s.builder.Set(ClusterRbacConfigTypeURL, "1", emptyRbacConfig)
			s.builder.Set(QuotaSpecTypeURL, "1", emptyQuotaSpec)
			s.builder.Set(QuotaSpecBindingTypeURL, "1", emptyQuotaSpecBinding)
			s.builder.Set(HTTPAPISpecBindingTypeURL, "1", emptyHTTPAPISpecBinding)
			s.builder.Set(HTTPAPISpecTypeURL, "1", emptyHTTPAPISpec)
			s.builder.Set(ServiceRoleBindingTypeURL, "1", emptyServiceRoleBinding)
			s.builder.Set(ServiceRoleTypeURL, "1", emptyServiceRole)
			s.builder.Set(PolicyTypeURL, "1", emptyPolicy)
			s.builder.Set(MeshPolicyTypeURL, "1", emptyPolicy)
			s.builder.Set(EnvoyFilterTypeURL, "1", emptyEnvoyFilter)


			fmt.Println("about to set snap")
			shot := s.builder.Build()
			fmt.Printf("Snapshot is this: %+v", shot)
			s.setter.SetSnapshot(node, shot)
			s.builder = shot.Builder()
		}
	}
}

func (s *Snapshot) version() string {
	return strconv.Itoa(s.ver)
}

func (s *Snapshot) increment() string {
	s.ver++
	return s.version()
}

func getTypeURLByType(name string) string {
	protoSchema, ok := model.IstioConfigTypes.GetByType(name)
	if !ok {
		fmt.Fprintf(os.Stdout, "Istio Config Type %q does not exist.\n", name)
		os.Exit(1)
	}

	return protoSchema.Collection
}

func init() {
	DestinationRuleTypeURL = getTypeURLByType("destination-rule")
	VirtualServiceTypeURL = getTypeURLByType("virtual-service")
	GatewayTypeURL = getTypeURLByType("gateway")
	ServiceEntryTypeURL = getTypeURLByType("service-entry")
	EnvoyFilterTypeURL = getTypeURLByType("envoy-filter")
	SidecarTypeURL = getTypeURLByType("sidecar")
	HTTPAPISpecTypeURL = getTypeURLByType("http-api-spec")
	HTTPAPISpecBindingTypeURL = getTypeURLByType("http-api-spec-binding")
	QuotaSpecTypeURL = getTypeURLByType("quota-spec")
	QuotaSpecBindingTypeURL = getTypeURLByType("quota-spec-binding")
	PolicyTypeURL = getTypeURLByType("policy")
	MeshPolicyTypeURL = getTypeURLByType("mesh-policy")
	ServiceRoleTypeURL = getTypeURLByType("service-role")
	ServiceRoleBindingTypeURL = getTypeURLByType("service-role-binding")
	RbacConfigTypeURL = getTypeURLByType("rbac-config")
	ClusterRbacConfigTypeURL = getTypeURLByType("cluster-rbac-config")
}
