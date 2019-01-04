package snapshot

import (
	"os"
	"reflect"
	"strconv"
	"time"

	"code.cloudfoundry.org/copilot/models"
	"code.cloudfoundry.org/lager"

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
	SidecarTypeURL            = "type.googleapis.com/istio.networking.v1alpha3.Sidecar"
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
			sidecars := s.config.CreateSidecarEnvelopes(routes, newVersion)

			s.builder.Set(GatewayTypeURL, "1", gateways)
			s.builder.Set(VirtualServiceTypeURL, newVersion, virtualServices)
			s.builder.Set(DestinationRuleTypeURL, newVersion, destinationRules)
			s.builder.Set(ServiceEntryTypeURL, newVersion, serviceEntries)
			s.builder.Set(SidecarTypeURL, newVersion, sidecars)

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
