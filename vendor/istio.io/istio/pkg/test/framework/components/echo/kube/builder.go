// Copyright 2019 Istio Authors
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

package kube

import (
	"sync"

	"github.com/hashicorp/go-multierror"

	"istio.io/istio/pkg/test"
	"istio.io/istio/pkg/test/framework/components/echo"
	kubeEnv "istio.io/istio/pkg/test/framework/components/environment/kube"
	"istio.io/istio/pkg/test/framework/resource"

	kubeCore "k8s.io/api/core/v1"
)

var _ echo.Builder = &builder{}

type builder struct {
	ctx        resource.Context
	references []*echo.Instance
	configs    []echo.Config
}

func NewBuilder(ctx resource.Context) echo.Builder {
	return &builder{
		ctx: ctx,
	}
}

func (b *builder) With(i *echo.Instance, cfg echo.Config) echo.Builder {
	b.references = append(b.references, i)
	b.configs = append(b.configs, cfg)
	return b
}

func (b *builder) Build() error {
	instances, err := b.newInstances()
	if err != nil {
		return err
	}

	if err := b.initializeInstances(instances); err != nil {
		return err
	}

	if err := b.waitUntilAllCallable(instances); err != nil {
		return err
	}

	// Success... update the caller's references.
	for i, inst := range instances {
		*b.references[i] = inst
	}
	return nil
}

func (b *builder) BuildOrFail(t test.Failer) {
	t.Helper()
	if err := b.Build(); err != nil {
		t.Fatal(err)
	}
}

func (b *builder) newInstances() ([]echo.Instance, error) {
	instances := make([]echo.Instance, 0, len(b.configs))
	for _, cfg := range b.configs {
		inst, err := newInstance(b.ctx, cfg)
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func (b *builder) initializeInstances(instances []echo.Instance) error {
	env := b.ctx.Environment().(*kubeEnv.Environment)

	// Wait to receive the k8s Endpoints for each Echo Instance.
	wg := sync.WaitGroup{}
	instanceEndpoints := make([]*kubeCore.Endpoints, len(instances))
	aggregateErrMux := &sync.Mutex{}
	var aggregateErr error
	for i, inst := range instances {
		wg.Add(1)

		instanceIndex := i
		serviceName := inst.Config().Service
		serviceNamespace := inst.Config().Namespace.Name()

		// Run the waits in parallel.
		go func() {
			defer wg.Done()

			// Wait until all the endpoints are ready for this service
			_, endpoints, err := env.WaitUntilServiceEndpointsAreReady(serviceNamespace, serviceName)
			if err != nil {
				aggregateErrMux.Lock()
				aggregateErr = multierror.Append(aggregateErr, err)
				aggregateErrMux.Unlock()
				return
			}
			instanceEndpoints[instanceIndex] = endpoints
		}()
	}

	wg.Wait()

	if aggregateErr != nil {
		return aggregateErr
	}

	// Initialize the workloads for each instance.
	for i, inst := range instances {
		if err := inst.(*instance).initialize(instanceEndpoints[i]); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) waitUntilAllCallable(instances []echo.Instance) error {
	// Now wait for each endpoint to be callable from all others.
	wg := sync.WaitGroup{}
	aggregateErrMux := &sync.Mutex{}
	var aggregateErr error
	for _, inst := range instances {
		wg.Add(1)

		source := inst
		go func() {
			defer wg.Done()

			if err := source.WaitUntilCallable(instances...); err != nil {
				aggregateErrMux.Lock()
				aggregateErr = multierror.Append(aggregateErr, err)
				aggregateErrMux.Unlock()
			}
		}()
	}
	wg.Wait()

	return aggregateErr
}
