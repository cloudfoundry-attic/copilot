package snapshot

import (
	"fmt"
	"os"
	"reflect"
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

			gateways := s.config.CreateGatewayEnvelopes()
			virtualServices := s.config.CreateVirtualServiceEnvelopes(routes, newVersion)
			destinationRules := s.config.CreateDestinationRuleEnvelopes(routes, newVersion)
			serviceEntries := s.config.CreateServiceEntryEnvelopes(routes, newVersion)

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
