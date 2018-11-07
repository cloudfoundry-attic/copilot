// Copyright 2018 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"
	"strconv"
	"sync/atomic"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"

	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/pool"
	"istio.io/istio/mixer/pkg/runtime/monitoring"
	"istio.io/istio/pkg/log"
)

type env struct {
	logger           adapter.Logger
	gp               *pool.GoroutinePool
	monitoringCtx    context.Context
	daemons, workers *int64
}

// NewEnv returns a new environment instance.
func NewEnv(cfgID int64, name string, gp *pool.GoroutinePool) adapter.Env {
	ctx := context.Background()
	var err error
	if ctx, err = tag.New(ctx, tag.Insert(monitoring.InitConfigIDTag, strconv.FormatInt(cfgID, 10)), tag.Insert(monitoring.HandlerTag, name)); err != nil {
		log.Errorf("could not setup context for stats: %v", err)
	}

	return env{
		logger:        newLogger(name),
		gp:            gp,
		monitoringCtx: ctx,
		daemons:       new(int64),
		workers:       new(int64),
	}
}

// Logger from adapter.Env.
func (e env) Logger() adapter.Logger {
	return e.logger
}

// ScheduleWork from adapter.Env.
func (e env) ScheduleWork(fn adapter.WorkFunc) {
	stats.Record(e.monitoringCtx, monitoring.WorkersTotal.M(atomic.AddInt64(e.workers, 1)))

	// TODO (Issue #2503): This method creates a closure which causes allocations. We can ensure that we're
	// not creating a closure by calling a method by name, instead of using an anonymous one.
	e.gp.ScheduleWork(func(ifn interface{}) {
		reachedEnd := false

		defer func() {
			// Always decrement the worker count.
			stats.Record(e.monitoringCtx, monitoring.WorkersTotal.M(atomic.AddInt64(e.workers, -1)))

			if !reachedEnd {
				r := recover()
				_ = e.Logger().Errorf("Adapter worker failed: %v", r) // nolint: gas

				// TODO (Issue #2503): Beyond logging, we want to do something proactive here.
				//       For example, we want to probably terminate the originating
				//       adapter and record the failure so we can count how often
				//       it happens, etc.
			}
		}()

		ifn.(adapter.WorkFunc)()
		reachedEnd = true
	}, fn)
}

// ScheduleDaemon from adapter.Env.
func (e env) ScheduleDaemon(fn adapter.DaemonFunc) {
	stats.Record(e.monitoringCtx, monitoring.DaemonsTotal.M(atomic.AddInt64(e.daemons, 1)))

	go func() {
		reachedEnd := false

		defer func() {
			// Always decrement the daemon count.
			stats.Record(e.monitoringCtx, monitoring.DaemonsTotal.M(atomic.AddInt64(e.daemons, -1)))

			if !reachedEnd {
				r := recover()
				_ = e.Logger().Errorf("Adapter daemon failed: %v", r) // nolint: gas

				// TODO (Issue #2503): Beyond logging, we want to do something proactive here.
				//       For example, we want to probably terminate the originating
				//       adapter and record the failure so we can count how often
				//       it happens, etc.
			}
		}()

		fn()
		reachedEnd = true
	}()
}

func (e env) Workers() int64 {
	return atomic.LoadInt64(e.workers)
}

func (e env) Daemons() int64 {
	return atomic.LoadInt64(e.daemons)
}

func (e env) reportStrayWorkers() error {
	if atomic.LoadInt64(e.daemons) > 0 {
		// TODO: ideally we should return some sort of error here to bubble up this issue to the top so that
		// operator can look at it. However, currently we cannot guarantee that SchedulerXXXX gauge
		// counter will give consistent value because of timing issue in the ScheduleWorker and ScheduleDaemon.
		// Basically, even if the adapter would have closed everything before returning from Close function, our
		// counter might get delayed decremented, causing this false positive error.
		// Therefore, we need a new retry kind logic on handler Close to give time for counters to get updated
		// before making this as a red flag error. runtime work has plans to implement this stuff, we can revisit
		// this to-do then. Same for the code below related to workers.
		_ = e.Logger().Errorf("adapter did not close all the scheduled daemons")
	}

	if atomic.LoadInt64(e.workers) > 0 {
		_ = e.Logger().Errorf("adapter did not close all the scheduled workers")
	}

	return nil
}
