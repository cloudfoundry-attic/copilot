package models

import (
	"os"
	"reflect"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/events"
	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type sets struct {
	External *BackendSet
	Internal *BackendSet
}

type store struct {
	sync.RWMutex
	content map[DiegoProcessGUID]sets
}

type BackendSet struct {
	Backends []*Backend
}

type Backend struct {
	Address       string
	Port          uint32
	ContainerPort uint32
}

type RouteWithBackends struct {
	Hostname        string
	Path            string
	Backends        BackendSet
	CapiProcessGUID string
	RouteWeight     int32
	Internal        bool
	VIP             string
}

func (s *store) Insert(guid DiegoProcessGUID, isInternal bool, additionalBackend *Backend) {
	if additionalBackend == nil {
		return
	}

	s.Lock()
	if _, ok := s.content[guid]; !ok {
		s.content[guid] = sets{
			External: &BackendSet{},
			Internal: &BackendSet{},
		}
	}

	backends := s.content[guid].External.Backends
	if isInternal {
		backends = s.content[guid].Internal.Backends
	}

	s.Unlock()

	for _, backend := range backends {
		if reflect.DeepEqual(backend, additionalBackend) {
			return
		}
	}

	s.Lock()
	if isInternal {
		s.content[guid].Internal.Backends = append(s.content[guid].Internal.Backends, additionalBackend)
	} else {
		s.content[guid].External.Backends = append(s.content[guid].External.Backends, additionalBackend)
	}
	s.Unlock()
}

func (s *store) Remove(guid DiegoProcessGUID) {
	s.Lock()
	defer s.Unlock()

	delete(s.content, guid)
}

type DiegoBackendSetRepo struct {
	bbs    BBSEventer
	logger lager.Logger
	ticker <-chan time.Time
	store  store
}

//go:generate counterfeiter -o fakes/bbs_eventer.go --fake-name BBSEventer . BBSEventer
type BBSEventer interface {
	SubscribeToInstanceEvents(logger lager.Logger) (events.EventSource, error)
	ActualLRPs(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRP, error)
}

type BackendSetRepo interface {
	Run(signals <-chan os.Signal, ready chan<- struct{}) error
	Get(guid DiegoProcessGUID) *BackendSet
	GetInternalBackends(guid DiegoProcessGUID) *BackendSet
}

func NewBackendSetRepo(bbs BBSEventer, logger lager.Logger, tChan <-chan time.Time) BackendSetRepo {
	if bbs == nil {
		logger.Info("BBS support is disabled, using no-op backend")
		return &NoopBackendSetRepo{}
	}
	return &DiegoBackendSetRepo{
		bbs:    bbs,
		logger: logger,
		ticker: tChan,
		store: store{
			content: make(map[DiegoProcessGUID]sets),
		},
	}
}

func (b *DiegoBackendSetRepo) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	stop := make(chan struct{})

	eventSource, err := b.bbs.SubscribeToInstanceEvents(b.logger)
	if err != nil {
		return err
	}

	go b.collectEvents(stop, eventSource)
	go b.reconcileLRPs(stop, b.ticker)

	close(ready)

	for {
		select {
		case <-signals:
			close(stop)
			return nil
		}
	}
}

func (b *DiegoBackendSetRepo) Get(guid DiegoProcessGUID) *BackendSet {
	b.store.RLock()
	defer b.store.RUnlock()

	if val, ok := b.store.content[guid]; ok {
		return val.External
	}
	return &BackendSet{[]*Backend{}}
}

func (b *DiegoBackendSetRepo) GetInternalBackends(guid DiegoProcessGUID) *BackendSet {
	b.store.RLock()
	defer b.store.RUnlock()

	if val, ok := b.store.content[guid]; ok {
		return val.Internal
	}
	return &BackendSet{[]*Backend{}}
}

func (b *DiegoBackendSetRepo) collectEvents(stop <-chan struct{}, eventSource events.EventSource) {
	for {
		select {
		case <-stop:
			b.logger.Info("events-exit")
			return
		default:
			event, err := eventSource.Next()
			if err != nil {
				b.logger.Debug("events-next", lager.Data{"events-error": err.Error()})
				continue
			}

			switch event := event.(type) {
			case *bbsmodels.ActualLRPInstanceCreatedEvent:
				lrp := event.GetActualLrp()
				ex, in := processActualLRP(lrp)
				guid := DiegoProcessGUID(lrp.GetProcessGuid())
				b.store.Insert(guid, false, ex)
				b.store.Insert(guid, true, in)

			case *bbsmodels.ActualLRPInstanceRemovedEvent:
				guid := DiegoProcessGUID(event.GetActualLrp().GetProcessGuid())
				b.store.Remove(guid)
			default:
				b.logger.Debug("unhandled-event-type")
				return
			}
		}
	}
}

func (b *DiegoBackendSetRepo) reconcileLRPs(stop <-chan struct{}, ticker <-chan time.Time) {
	for {
		select {
		case <-ticker:
			lrps, err := b.bbs.ActualLRPs(b.logger, bbsmodels.ActualLRPFilter{})
			if err != nil {
				b.logger.Debug("lrp-groups-error", lager.Data{"lrp-groups-error": err.Error()})
				continue
			}

			// not locking replacement store - no other goroutine can update it
			replaceStore := store{content: make(map[DiegoProcessGUID]sets)}
			for _, lrp := range lrps {
				ex, in := processActualLRP(lrp)
				guid := DiegoProcessGUID(lrp.GetProcessGuid())
				replaceStore.Insert(guid, false, ex)
				replaceStore.Insert(guid, true, in)
			}

			b.store.Lock()
			b.store.content = replaceStore.content
			b.store.Unlock()
		case <-stop:
			b.logger.Info("lrp-groups-exit")
			return
		}
	}
}

func processActualLRP(lrp *bbsmodels.ActualLRP) (*Backend, *Backend) {
	var (
		appHostPort      uint32
		appContainerPort uint32
		externalBackend  *Backend
		internalBackend  *Backend
	)

	if lrp.GetPresence() == bbsmodels.ActualLRP_Evacuating {
		return externalBackend, internalBackend
	}

	if lrp.GetState() != bbsmodels.ActualLRPStateRunning {
		return externalBackend, internalBackend
	}

	for _, port := range lrp.ActualLRPNetInfo.Ports {
		if port.ContainerPort != CF_APP_SSH_PORT {
			appHostPort = port.HostPort
			appContainerPort = port.ContainerPort
		}
	}

	if appHostPort != 0 {
		externalBackend = &Backend{
			Address:       lrp.ActualLRPNetInfo.Address,
			Port:          appHostPort,
			ContainerPort: appContainerPort,
		}
	}

	if appContainerPort != 0 {
		internalBackend = &Backend{
			Address:       lrp.ActualLRPNetInfo.InstanceAddress,
			Port:          appContainerPort,
			ContainerPort: appContainerPort,
		}
	}

	return externalBackend, internalBackend
}
