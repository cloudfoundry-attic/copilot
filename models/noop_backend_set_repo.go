package models

import (
	"os"
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

func (b *NoopBackendSetRepo) Get(guid DiegoProcessGUID) *BackendSet {
	return nil
}

func (b *NoopBackendSetRepo) GetInternalBackends(guid DiegoProcessGUID) *BackendSet {
	return nil
}
