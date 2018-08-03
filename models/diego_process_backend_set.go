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
	events := make(chan bbsmodels.Event)
	stop := make(chan struct{})

	eventSource, err := b.bbs.SubscribeToEvents(b.logger)
	if err != nil {
		return err
	}

	go b.collectEvents(stop, eventSource, events)
	go b.reconcileLRPs(stop, b.ticker, events)

	close(ready)

	for {
		select {
		case <-signals:
			close(stop)
			return nil
		case event := <-events:
			b.processEvent(event)
		}
	}
}

func (b *BackendSetRepo) Get(guid DiegoProcessGUID) *api.BackendSet {
	b.store.RLock()
	defer b.store.RUnlock()
	return b.store.content[guid]
}

func (b *BackendSetRepo) collectEvents(stop <-chan struct{}, eventSource events.EventSource, events chan<- bbsmodels.Event) {
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

			events <- event
		}
	}
}

func (b *BackendSetRepo) reconcileLRPs(stop <-chan struct{}, ticker <-chan time.Time, events chan<- bbsmodels.Event) {
	for {
		select {
		case <-ticker:
			groups, err := b.bbs.ActualLRPGroups(b.logger, bbsmodels.ActualLRPFilter{})
			if err != nil {
				b.logger.Debug("lrp-groups-error", lager.Data{"lrp-groups-error": err.Error()})
			}

			for _, group := range groups {
				events <- bbsmodels.NewActualLRPCreatedEvent(group)
			}
		case <-stop:
			b.logger.Info("lrp-groups-exit")
			return
		}
	}
}

func (b *BackendSetRepo) processEvent(e bbsmodels.Event) {
	var instance *bbsmodels.ActualLRP
	switch e.EventType() {
	case bbsmodels.EventTypeActualLRPCreated:
		createdEvent := e.(*bbsmodels.ActualLRPCreatedEvent)
		instance = createdEvent.GetActualLrpGroup().GetInstance()
	case bbsmodels.EventTypeActualLRPRemoved:
		deletedEvent := e.(*bbsmodels.ActualLRPRemovedEvent)
		b.store.Lock()
		diegoProcessGUID := DiegoProcessGUID(deletedEvent.ActualLrpGroup.Instance.ActualLRPKey.ProcessGuid)
		delete(b.store.content, diegoProcessGUID)
		b.store.Unlock()
		return
	default:
		b.logger.Debug("unhandled-event-type")
		return
	}

	diegoProcessGUID := DiegoProcessGUID(instance.ActualLRPKey.ProcessGuid)
	if instance.State != bbsmodels.ActualLRPStateRunning {
		return
	}

	var appHostPort uint32
	for _, port := range instance.ActualLRPNetInfo.Ports {
		if port.ContainerPort != CF_APP_SSH_PORT {
			appHostPort = port.HostPort
		}
	}

	if appHostPort == 0 {
		return
	}

	b.store.Lock()
	defer b.store.Unlock()
	if _, ok := b.store.content[diegoProcessGUID]; !ok {
		b.store.content[diegoProcessGUID] = &api.BackendSet{}
	}

	backendToAdd := &api.Backend{
		Address: instance.ActualLRPNetInfo.Address,
		Port:    appHostPort,
	}

	backends := b.store.content[diegoProcessGUID].Backends
	for _, backend := range backends {
		if fmt.Sprintf("%s:%d", backend.Address, backend.Port) == fmt.Sprintf("%s:%d", backendToAdd.Address, backendToAdd.Port) {
			return
		}
	}

	b.store.content[diegoProcessGUID].Backends = append(b.store.content[diegoProcessGUID].Backends, backendToAdd)
}
