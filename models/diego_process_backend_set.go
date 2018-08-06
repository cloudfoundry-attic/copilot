package models

import (
	"fmt"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/events"
	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
)

type store struct {
	sync.RWMutex
	content map[DiegoProcessGUID]*api.BackendSet
}

func (s *store) Insert(guid DiegoProcessGUID, additionalBackend *api.Backend) {
	s.Lock()
	if _, ok := s.content[guid]; !ok {
		s.content[guid] = &api.BackendSet{}
	}

	backends := s.content[guid].Backends
	s.Unlock()

	for _, backend := range backends {
		if fmt.Sprintf("%s:%d", backend.Address, backend.Port) == fmt.Sprintf("%s:%d", additionalBackend.Address, additionalBackend.Port) {
			return
		}
	}

	s.Lock()
	s.content[guid].Backends = append(s.content[guid].Backends, additionalBackend)
	s.Unlock()
}

type BackendSetRepo struct {
	bbs    bbsEventer
	logger lager.Logger
	ticker <-chan time.Time
	store  store
}

//go:generate counterfeiter -o fakes/bbs_eventer.go --fake-name BBSEventer . bbsEventer
type bbsEventer interface {
	SubscribeToEvents(logger lager.Logger) (events.EventSource, error)
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

func NewBackendSetRepo(bbs bbsEventer, logger lager.Logger, ticker <-chan time.Time) *BackendSetRepo {
	return &BackendSetRepo{
		bbs:    bbs,
		logger: logger,
		ticker: ticker,
		store: store{
			content: make(map[DiegoProcessGUID]*api.BackendSet),
		},
	}
}

func (b *BackendSetRepo) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	stop := make(chan struct{})

	eventSource, err := b.bbs.SubscribeToEvents(b.logger)
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

func (b *BackendSetRepo) Get(guid DiegoProcessGUID) *api.BackendSet {
	b.store.RLock()
	defer b.store.RUnlock()
	return b.store.content[guid]
}

func (b *BackendSetRepo) collectEvents(stop <-chan struct{}, eventSource events.EventSource) {
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

			switch event.EventType() {
			case bbsmodels.EventTypeActualLRPCreated:
				createdEvent := event.(*bbsmodels.ActualLRPCreatedEvent)
				instance := createdEvent.GetActualLrpGroup().GetInstance()
				be := processInstance(instance)
				guid := DiegoProcessGUID(instance.ActualLRPKey.GetProcessGuid())
				b.store.Insert(guid, be)
			default:
				b.logger.Debug("unhandled-event-type")
				return
			}
		}
	}
}

func (b *BackendSetRepo) reconcileLRPs(stop <-chan struct{}, ticker <-chan time.Time) {
	for {
		select {
		case <-ticker:
			groups, err := b.bbs.ActualLRPGroups(b.logger, bbsmodels.ActualLRPFilter{})
			if err != nil {
				b.logger.Debug("lrp-groups-error", lager.Data{"lrp-groups-error": err.Error()})
				continue
			}

			// not locking replacement store - no other goroutine can update it
			replaceStore := store{content: make(map[DiegoProcessGUID]*api.BackendSet)}
			for _, group := range groups {
				be := processInstance(group.Instance)
				guid := DiegoProcessGUID(group.GetInstance().ActualLRPKey.GetProcessGuid())
				replaceStore.Insert(guid, be)
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

func processInstance(instance *bbsmodels.ActualLRP) *api.Backend {
	if instance.State != bbsmodels.ActualLRPStateRunning {
		return nil
	}

	var appHostPort uint32
	for _, port := range instance.ActualLRPNetInfo.Ports {
		if port.ContainerPort != CF_APP_SSH_PORT {
			appHostPort = port.HostPort
		}
	}

	if appHostPort == 0 {
		return nil
	}

	return &api.Backend{
		Address: instance.ActualLRPNetInfo.Address,
		Port:    appHostPort,
	}
}
