package models

import (
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
	ticker *time.Ticker
	store  store
}

//go:generate counterfeiter -o fakes/bbs_eventer.go --fake-name BBSEventer . bbsEventer
type bbsEventer interface {
	SubscribeToEvents(logger lager.Logger) (events.EventSource, error)
	ActualLRPGroups(lager.Logger, bbsmodels.ActualLRPFilter) ([]*bbsmodels.ActualLRPGroup, error)
}

func NewBackendSetRepo(bbs bbsEventer, logger lager.Logger, refreshDuration time.Duration) *BackendSetRepo {
	return &BackendSetRepo{
		bbs:    bbs,
		logger: logger,
		ticker: time.NewTicker(refreshDuration),
		store: store{
			content: make(map[DiegoProcessGUID]*api.BackendSet),
		},
	}
}

func (b *BackendSetRepo) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	lrps := make(chan *bbsmodels.ActualLRP)

	eventSource, err := b.bbs.SubscribeToEvents(b.logger)
	if err != nil {
		return err
	}

	go b.collectEvents(signals, eventSource, lrps)
	go b.reconcileLRPs(signals, b.ticker, lrps)

	close(ready)

	for {
		select {
		case <-signals:
			return nil
		case instance := <-lrps:
			b.createProcessGUIDToBackendSet(instance)
		}
	}
}

func (b *BackendSetRepo) Get(guid DiegoProcessGUID) *api.BackendSet {
	b.store.RLock()
	defer b.store.RUnlock()
	return b.store.content[guid]
}

func (b *BackendSetRepo) collectEvents(signals <-chan os.Signal, eventSource events.EventSource, lrps chan<- *bbsmodels.ActualLRP) {
	for {
		select {
		case <-signals:
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
				lrps <- createdEvent.GetActualLrpGroup().GetInstance()
			}
		}
	}
}

func (b *BackendSetRepo) reconcileLRPs(signals <-chan os.Signal, ticker *time.Ticker, lrps chan<- *bbsmodels.ActualLRP) {
	for {
		select {
		case <-ticker.C:
			groups, err := b.bbs.ActualLRPGroups(b.logger, bbsmodels.ActualLRPFilter{})
			if err != nil {
				b.logger.Debug("lrp-groups-error", lager.Data{"lrp-groups-error": err.Error()})
			}

			for _, group := range groups {
				lrps <- group.Instance
			}
		case <-signals:
			return
		}
	}
}

func (b *BackendSetRepo) createProcessGUIDToBackendSet(instance *bbsmodels.ActualLRP) {
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
	if _, ok := b.store.content[diegoProcessGUID]; !ok {
		b.store.content[diegoProcessGUID] = &api.BackendSet{}
	}

	b.store.content[diegoProcessGUID].Backends = append(b.store.content[diegoProcessGUID].Backends, &api.Backend{
		Address: instance.ActualLRPNetInfo.Address,
		Port:    appHostPort,
	})
	b.store.Unlock()
}
