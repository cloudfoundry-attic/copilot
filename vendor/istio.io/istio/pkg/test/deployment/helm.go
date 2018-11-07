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

package deployment

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"istio.io/istio/pkg/test/framework/scopes"
	"istio.io/istio/pkg/test/kube"
	"istio.io/istio/pkg/test/shell"
)

const (
	namespaceTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    istio-injection: disabled
`
)

// HelmConfig configuration for a Helm-based deployment.
type HelmConfig struct {
	Accessor   *kube.Accessor
	KubeConfig string
	Namespace  string
	WorkDir    string
	ChartDir   string

	// Can be either a file name under ChartDir or an absolute file path.
	ValuesFile string
	Values     map[string]string
}

// NewHelmDeployment creates a new Helm-based deployment instance.
func NewHelmDeployment(c HelmConfig) (*Instance, error) {
	instance := &Instance{}

	instance.kubeConfig = c.KubeConfig
	instance.namespace = c.Namespace

	// Define a deployment name for Helm.
	deploymentName := fmt.Sprintf("%s-%v", c.Namespace, time.Now().UnixNano())
	scopes.CI.Infof("Generated Helm Instance name: %s", deploymentName)

	instance.yamlFilePath = path.Join(c.WorkDir, deploymentName+".yaml")

	// Convert the valuesFile to an absolute file path.
	valuesFile := c.ValuesFile
	if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
		valuesFile = filepath.Join(c.ChartDir, valuesFile)
		if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
			return nil, err
		}
	}

	var err error
	var generatedYaml string
	if generatedYaml, err = HelmTemplate(
		deploymentName,
		c.Namespace,
		c.ChartDir,
		valuesFile,
		c.Values); err != nil {
		return nil, fmt.Errorf("chart generation failed: %v", err)
	}

	// TODO: This is Istio deployment specific. We may need to remove/reconcile this as a parameter
	// when we support Helm deployment of non-Istio artifacts.
	namespaceData := fmt.Sprintf(namespaceTemplate, c.Namespace)

	generatedYaml = namespaceData + generatedYaml

	if err = ioutil.WriteFile(instance.yamlFilePath, []byte(generatedYaml), os.ModePerm); err != nil {
		return nil, fmt.Errorf("unable to write helm generated yaml: %v", err)
	}

	scopes.CI.Infof("Applying Helm generated Yaml file: %s", instance.yamlFilePath)
	if err = kube.Apply(c.KubeConfig, c.Namespace, instance.yamlFilePath); err != nil {
		return nil, fmt.Errorf("kube apply of generated yaml filed: %v", err)
	}

	if err = instance.wait(c.Namespace, c.Accessor); err != nil {
		return nil, err
	}

	return instance, nil
}

// HelmTemplate calls "helm template".
func HelmTemplate(deploymentName, namespace, chartDir, valuesFile string, values map[string]string) (string, error) {
	valuesString := ""

	// Apply the overrides for the values file.
	if values != nil {
		for k, v := range values {
			valuesString += fmt.Sprintf(" --set %s=%s", k, v)
		}
	}

	valuesFileString := ""
	if valuesFile != "" {
		valuesFileString = fmt.Sprintf(" --values %s", valuesFile)
	}

	str, err := shell.Execute(
		"helm template %s --name %s --namespace %s%s%s",
		chartDir, deploymentName, namespace, valuesFileString, valuesString)
	if err == nil {
		return str, nil
	}

	return "", fmt.Errorf("%v: %s", err, str)
}
