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

package kube

import (
	"flag"
)

// init registers the command-line flags that we can exposed for "go test".
func init() {
	flag.StringVar(&globalSettings.KubeConfig, "istio.test.kube.config", globalSettings.KubeConfig,
		"The path to the kube config file for cluster environments")
	flag.StringVar(&globalSettings.SystemNamespace, "istio.test.kube.systemNamespace", globalSettings.SystemNamespace,
		"The namespace where the Istio components reside in a typical deployment (default: 'istio-system').")
	flag.StringVar(&globalSettings.SuiteNamespace, "istio.test.kube.suiteNamespace", globalSettings.SuiteNamespace,
		"The namespace in which non-system components with suite scope are deployed. If not specified, a new namespace "+
			"will be generated with a UUID once per run.")
	flag.StringVar(&globalSettings.TestNamespace, "istio.test.kube.testNamespace", globalSettings.TestNamespace,
		"The namespace in which non-system components with test scope are deployed. If not specified, the namespaces "+
			"are created when an environment is acquired in a test, and the previous one gets deleted. This ensures that "+
			"during a single test run, there is only one test namespace in the system.")
	flag.BoolVar(&globalSettings.DeployIstio, "istio.test.kube.deploy", globalSettings.DeployIstio,
		"Deploy Istio into the target Kubernetes environment.")
	flag.DurationVar(&globalSettings.DeployTimeout, "istio.test.kube.deployTimeout", globalSettings.DeployTimeout,
		"Timeout applied to deploying Istio into the target Kubernetes environment. Only applies if DeployIstio=true.")
	flag.DurationVar(&globalSettings.UndeployTimeout, "istio.test.kube.undeployTimeout", globalSettings.UndeployTimeout,
		"Timeout applied to undeploying Istio from the target Kubernetes environment. Only applies if DeployIstio=true.")
	flag.BoolVar(&globalSettings.MinikubeIngress, "istio.test.kube.minikubeingress", globalSettings.MinikubeIngress,
		"Configure the Ingress component so that it gets the IP address from Node, when Minikube is used..")
	flag.StringVar(&globalSettings.ChartDir, "istio.test.kube.helm.chartDir", globalSettings.ChartDir,
		"Helm chart dir for Istio. Only valid when deploying Istio.")
	flag.StringVar(&globalSettings.ValuesFile, "istio.test.kube.helm.valuesFile", globalSettings.ValuesFile,
		"Helm values file. This can be an absolute path or relative to chartDir. Only valid when deploying Istio.")
	flag.StringVar(&helmValues, "istio.test.kube.helm.values", helmValues,
		"Manual overrides for Helm values file. Only valid when deploying Istio.")
}
