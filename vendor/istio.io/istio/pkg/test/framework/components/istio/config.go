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

package istio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"

	yaml2 "gopkg.in/yaml.v2"

	"istio.io/istio/pkg/test"
	"istio.io/istio/pkg/test/env"
	"istio.io/istio/pkg/test/framework/core/image"
	"istio.io/istio/pkg/test/framework/resource"
	"istio.io/istio/pkg/test/util/file"

	kubeCore "k8s.io/api/core/v1"
)

const (
	// DefaultSystemNamespace default value for SystemNamespace
	DefaultSystemNamespace = "istio-system"

	// ValuesMcpFile for Istio Helm deployment.
	E2EValuesFile = "test-values/values-e2e.yaml"

	// DefaultDeployTimeout for Istio
	DefaultDeployTimeout = time.Second * 300

	// TODO(https://github.com/istio/istio/issues/12606): This timeout is insanely large, but Prow seems to take a lot of time
	//  pulling images.
	// DefaultCIDeployTimeout for Istio
	DefaultCIDeployTimeout = time.Minute * 20

	// DefaultUndeployTimeout for Istio.
	DefaultUndeployTimeout = time.Second * 300

	// DefaultCIUndeployTimeout for Istio.
	DefaultCIUndeployTimeout = time.Second * 900

	// DefaultIstioChartRepo for Istio.
	DefaultIstioChartRepo = "https://gcsweb.istio.io/gcs/istio-prerelease/daily-build/release-1.1-latest-daily/charts/"
)

var (
	helmValues string

	settingsFromCommandline = &Config{
		ChartRepo:          DefaultIstioChartRepo,
		SystemNamespace:    DefaultSystemNamespace,
		IstioNamespace:     DefaultSystemNamespace,
		ConfigNamespace:    DefaultSystemNamespace,
		TelemetryNamespace: DefaultSystemNamespace,
		PolicyNamespace:    DefaultSystemNamespace,
		IngressNamespace:   DefaultSystemNamespace,
		EgressNamespace:    DefaultSystemNamespace,
		DeployIstio:        true,
		DeployTimeout:      0,
		UndeployTimeout:    0,
		ChartDir:           env.IstioChartDir,
		CrdsFilesDir:       env.CrdsFilesDir,
		ValuesFile:         E2EValuesFile,
	}
)

// Config provide kube-specific Config from flags.
type Config struct {
	// The namespace where the Istio components (<=1.1) reside in a typical deployment (default: "istio-system").
	SystemNamespace string

	// The namespace in which istio ca and cert provisioning components are deployed.
	IstioNamespace string

	// The namespace in which config, discovery and auto-injector are deployed.
	ConfigNamespace string

	// The namespace in which mixer, kiali, tracing providers, graphana, prometheus are deployed.
	TelemetryNamespace string

	// The namespace in which istio policy checker is deployed.
	PolicyNamespace string

	// The namespace in which istio ingressgateway is deployed
	IngressNamespace string

	// The namespace in which istio egressgateway is deployed
	EgressNamespace string

	// Indicates that the test should deploy Istio into the target Kubernetes cluster before running tests.
	DeployIstio bool

	// DeployTimeout the timeout for deploying Istio.
	DeployTimeout time.Duration

	// UndeployTimeout the timeout for undeploying Istio.
	UndeployTimeout time.Duration

	ChartRepo string

	// The top-level Helm chart dir.
	ChartDir string

	// The top-level Helm Crds files dir.
	CrdsFilesDir string

	// The Helm values file to be used.
	ValuesFile string

	// Overrides for the Helm values file.
	Values map[string]string
}

// Is mtls enabled. Check in Values flag and Values file.
func (c *Config) IsMtlsEnabled() bool {
	if c.Values["global.mtls.enabled"] == "true" {
		return true
	}

	data, err := file.AsString(filepath.Join(c.ChartDir, c.ValuesFile))
	if err != nil {
		return false
	}
	m := make(map[interface{}]interface{})
	err = yaml2.Unmarshal([]byte(data), &m)
	if err != nil {
		return false
	}
	if m["global"] != nil {
		switch globalVal := m["global"].(type) {
		case map[interface{}]interface{}:
			switch mtlsVal := globalVal["mtls"].(type) {
			case map[interface{}]interface{}:
				return mtlsVal["enabled"].(bool)
			}
		}
	}

	return false
}

// DefaultConfig creates a new Config from defaults, environments variables, and command-line parameters.
func DefaultConfig(ctx resource.Context) (Config, error) {
	// Make a local copy.
	s := *settingsFromCommandline

	if err := normalizeFile(&s.ChartDir); err != nil {
		return Config{}, err
	}

	if err := checkFileExists(filepath.Join(s.ChartDir, s.ValuesFile)); err != nil {
		return Config{}, err
	}

	if err := normalizeFile(&s.CrdsFilesDir); err != nil {
		return Config{}, err
	}

	deps, err := image.SettingsFromCommandLine()
	if err != nil {
		return Config{}, err
	}

	if s.Values, err = newHelmValues(deps); err != nil {
		return Config{}, err
	}

	if ctx.Settings().CIMode {
		s.DeployTimeout = DefaultCIDeployTimeout
		s.UndeployTimeout = DefaultCIUndeployTimeout
	} else {
		s.DeployTimeout = DefaultDeployTimeout
		s.UndeployTimeout = DefaultUndeployTimeout
	}

	return s, nil
}

// DefaultConfigOrFail calls DefaultConfig and fails t if an error occurs.
func DefaultConfigOrFail(t test.Failer, ctx resource.Context) Config {
	cfg, err := DefaultConfig(ctx)
	if err != nil {
		t.Fatalf("Get istio config: %v", err)
	}
	return cfg
}

func normalizeFile(path *string) error {
	// If the path uses the homedir ~, expand the path.
	var err error
	*path, err = homedir.Expand(*path)
	if err != nil {
		return err
	}

	return checkFileExists(*path)
}

func checkFileExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}
	return nil
}

func newHelmValues(s *image.Settings) (map[string]string, error) {
	userValues, err := parseHelmValues()
	if err != nil {
		return nil, err
	}

	// Copy the defaults first.
	values := make(map[string]string)

	// Common values
	values[image.HubValuesKey] = s.Hub
	values[image.TagValuesKey] = s.Tag
	values[image.ImagePullPolicyValuesKey] = s.PullPolicy

	// Copy the user values.
	for k, v := range userValues {
		values[k] = v
	}

	// Always pull Docker images if using the "latest".
	if values[image.TagValuesKey] == image.LatestTag {
		values[image.ImagePullPolicyValuesKey] = string(kubeCore.PullAlways)
	}
	return values, nil
}

func parseHelmValues() (map[string]string, error) {
	out := make(map[string]string)
	if helmValues == "" {
		return out, nil
	}

	values := strings.Split(helmValues, ",")
	for _, v := range values {
		parts := strings.Split(v, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("failed parsing helm values: %s", helmValues)
		}
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return out, nil
}

// String implements fmt.Stringer
func (c *Config) String() string {
	result := ""

	result += fmt.Sprintf("SystemNamespace:    %s\n", c.SystemNamespace)
	result += fmt.Sprintf("IstioNamespace:     %s\n", c.IstioNamespace)
	result += fmt.Sprintf("ConfigNamespace:    %s\n", c.ConfigNamespace)
	result += fmt.Sprintf("TelemetryNamespace: %s\n", c.TelemetryNamespace)
	result += fmt.Sprintf("PolicyNamespace:    %s\n", c.PolicyNamespace)
	result += fmt.Sprintf("IngressNamespace:   %s\n", c.IngressNamespace)
	result += fmt.Sprintf("EgressNamespace:    %s\n", c.EgressNamespace)
	result += fmt.Sprintf("DeployIstio:        %v\n", c.DeployIstio)
	result += fmt.Sprintf("DeployTimeout:      %s\n", c.DeployTimeout.String())
	result += fmt.Sprintf("UndeployTimeout:    %s\n", c.UndeployTimeout.String())
	result += fmt.Sprintf("Values:             %v\n", c.Values)

	return result
}
