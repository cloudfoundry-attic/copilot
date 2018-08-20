package snapshot

import (
	"os"
	"time"

	"code.cloudfoundry.org/copilot/api"
	"code.cloudfoundry.org/lager"
)

type Snapshot struct {
	logger    lager.Logger
	ticker    <-chan time.Time
	collector collector
}

//go:generate counterfeiter -o fakes/collector.go --fake-name Collector . collector
type collector interface {
	Collect() []api.RouteWithBackends
}

func NewSnapshot(logger lager.Logger, ticker <-chan time.Time, collector collector) *Snapshot {
	return &Snapshot{
		logger:    logger,
		ticker:    ticker,
		collector: collector,
	}
}

func (s *Snapshot) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	stop := make(chan struct{})

	close(ready)

	for {
		select {
		case <-signals:
			close(stop)
			return nil
		}
	}
}
