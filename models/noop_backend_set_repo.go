package models

import (
	"os"

	"code.cloudfoundry.org/copilot/api"
)

type NoopBackendSetRepo struct{}

func (b *NoopBackendSetRepo) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	for {
		select {
		case <-signals:
			return nil
		}
	}
}

func (b *NoopBackendSetRepo) Get(guid DiegoProcessGUID) *api.BackendSet {
	return nil
}

func (b *NoopBackendSetRepo) GetInternalBackends(guid DiegoProcessGUID) *api.BackendSet {
	return nil
}
