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

package native

import (
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/go-multierror"

	"istio.io/istio/pkg/test"
	"istio.io/istio/pkg/test/echo/client"
	"istio.io/istio/pkg/test/framework/components/echo"
	"istio.io/istio/pkg/test/framework/components/echo/common"
	"istio.io/istio/pkg/test/framework/components/environment/native"
	"istio.io/istio/pkg/test/framework/resource"
	"istio.io/istio/pkg/test/scopes"
)

var (
	_ echo.Instance = &instance{}
	_ io.Closer     = &instance{}
)

type instance struct {
	id       resource.ID
	config   echo.Config
	workload *workload
}

func newInstance(ctx resource.Context, cfg echo.Config) (out echo.Instance, err error) {
	env := ctx.Environment().(*native.Environment)

	// Fill in defaults for any missing values.
	if err = common.FillInDefaults(ctx, env.Domain, &cfg); err != nil {
		return nil, err
	}

	c := &instance{
		config: cfg,
	}
	c.id = ctx.TrackResource(c)

	// Create the workload for this configuration and assign ports.
	c.workload, err = newWorkload(ctx, &c.config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *instance) ID() resource.ID {
	return c.id
}

func (c *instance) WaitUntilCallable(instances ...echo.Instance) error {
	// No need to check for inbound readiness, since inbound ports for the native echo instance
	// are configured by bootstrap.

	if c.workload.sidecar == nil {
		// No sidecar, nothing to do.
		return nil
	}

	return c.workload.sidecar.WaitForConfig(common.OutboundConfigAcceptFunc(instances...))
}

func (c *instance) WaitUntilCallableOrFail(t test.Failer, instances ...echo.Instance) {
	t.Helper()
	if err := c.WaitUntilCallable(instances...); err != nil {
		t.Fatal(err)
	}
}

func (c *instance) Address() string {
	return localhost
}

func (c *instance) Config() echo.Config {
	return c.config
}

func (c *instance) Workloads() ([]echo.Workload, error) {
	return []echo.Workload{c.workload}, nil
}

func (c *instance) WorkloadsOrFail(t test.Failer) []echo.Workload {
	t.Helper()
	out, err := c.Workloads()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (c *instance) Call(opts echo.CallOptions) (client.ParsedResponses, error) {
	out, err := c.workload.Call(&opts)
	if err != nil {
		if opts.Port != nil {
			err = fmt.Errorf("failed calling %s->'%s://%s:%d/%s': %v",
				c.Config().Service,
				strings.ToLower(string(opts.Port.Protocol)),
				opts.Target.Config().Service,
				opts.Port.ServicePort,
				opts.Path,
				err)
		}
		return nil, err
	}
	return out, nil
}

func (c *instance) CallOrFail(t test.Failer, opts echo.CallOptions) client.ParsedResponses {
	t.Helper()
	r, err := c.Call(opts)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (c *instance) Close() (err error) {
	if c.workload != nil {
		scopes.Framework.Debugf("%s closing Echo workload", c.id)
		err = multierror.Append(err, c.workload.Close()).ErrorOrNil()
	}

	scopes.Framework.Debugf("%s close complete (err:%v)", c.id, err)
	return
}
