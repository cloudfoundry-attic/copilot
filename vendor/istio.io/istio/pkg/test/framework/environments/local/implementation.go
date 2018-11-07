//  Copyright 2018 Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package local

import (
	"time"

	meshConfig "istio.io/api/mesh/v1alpha1"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/test/framework/environment"
	"istio.io/istio/pkg/test/framework/environments/local/service"
	"istio.io/istio/pkg/test/framework/internal"
	"istio.io/istio/pkg/test/framework/scopes"
	"istio.io/istio/pkg/test/framework/settings"
	"istio.io/istio/pkg/test/framework/tmpl"
)

// Implementation of a local environment for testing. It implements environment.Interface, and also
// hosts publicly accessible methods that are specific to local environment.
type Implementation struct {
	ctx *internal.TestContext

	// Mesh for configuring pilot.
	Mesh *meshConfig.MeshConfig

	// ServiceManager for all deployed services.
	ServiceManager *service.Manager

	// The namespace where the Istio components reside in the local deployment.
	IstioSystemNamespace string
}

var _ environment.Implementation = &Implementation{}
var _ internal.EnvironmentController = &Implementation{}

// New returns a new instance of cluster environment.
func New() *Implementation {
	mesh := model.DefaultMeshConfig()
	return &Implementation{
		IstioSystemNamespace: service.Namespace,
		Mesh:                 &mesh,
		ServiceManager:       service.NewManager(),
	}
}

// EnvironmentID is the name of this environment implementation.
func (e *Implementation) EnvironmentID() settings.EnvironmentID {
	return settings.Local
}

// Initialize the environment. This is called once during the lifetime of the suite.
func (e *Implementation) Initialize(ctx *internal.TestContext) error {
	e.ctx = ctx
	return nil
}

// Configure applies the given configuration to the mesh.
func (e *Implementation) Configure(config string) error {
	for _, d := range e.ctx.Tracker.All() {
		if configurable, ok := d.(internal.Configurable); ok {
			err := configurable.ApplyConfig(config)
			if err != nil {
				return err
			}
		}
	}
	// TODO: Implement a mechanism for reliably waiting for the configuration to disseminate in the system.
	// We can use CtrlZ to expose the config state of Mixer and Pilot.
	// See https://github.com/istio/istio/issues/6169 and https://github.com/istio/istio/issues/6170.

	time.Sleep(time.Second * 2)
	scopes.Framework.Debugf("Completing sleep after configure step")
	return nil
}

// Evaluate the template against standard set of parameters
func (e *Implementation) Evaluate(template string) (string, error) {
	// For the local environment, just run everything in a single virtual namespace.
	p := tmpl.Parameters{
		IstioSystemNamespace: e.IstioSystemNamespace,
		TestNamespace:        e.IstioSystemNamespace,
		DependencyNamespace:  e.IstioSystemNamespace,
	}

	return tmpl.Evaluate(template, p)
}

// Reset the environment before starting another test.
func (e *Implementation) Reset() error {
	return nil
}

// DumpState dumps the state of the environment to the file system and the log.
func (e *Implementation) DumpState(context string) {
	// Nothing to do for local environment.
}

// CreateTmpDirectory creates a local temporary directory.
func (e *Implementation) CreateTmpDirectory(name string) (string, error) {
	return internal.CreateTmpDirectory(e.ctx.Settings().WorkDir, e.ctx.Settings().RunID, name)
}
