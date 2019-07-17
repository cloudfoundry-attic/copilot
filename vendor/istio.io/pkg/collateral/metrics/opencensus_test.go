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

package metrics_test

import (
	"reflect"
	"testing"

	"istio.io/pkg/collateral/metrics"
	"istio.io/pkg/monitoring"
)

func TestExportedMetrics(t *testing.T) {
	r := metrics.NewOpenCensusRegistry()
	if got := r.ExportedMetrics(); !reflect.DeepEqual(got, want) {
		t.Errorf("ExportedMetrics() = %v, want %v", got, want)
	}
}

var (
	// AttributesTotal is a measure of the number of known attributes.
	AttributesTotal = monitoring.NewGauge(
		"mixer/config/attributes_total",
		"The number of known attributes in the current config.",
	)

	// HandlersTotal is a measure of the number of known handlers.
	HandlersTotal = monitoring.NewGauge(
		"mixer/config/handler_configs_total",
		"The number of known handlers in the current config.",
	)

	// InstancesTotal is a measure of the number of known instances.
	InstancesTotal = monitoring.NewGauge(
		"mixer/config/instance_configs_total",
		"The number of known instances in the current config.",
	)

	// InstanceErrs is a measure of the number of errors for processing instance config.
	InstanceErrs = monitoring.NewGauge(
		"mixer/config/instance_config_errors_total",
		"The number of errors encountered during processing of the instance configuration.",
	)

	// RulesTotal is a measure of the number of known rules.
	RulesTotal = monitoring.NewGauge(
		"mixer/config/rule_configs_total",
		"The number of known rules in the current config.",
	)

	// RuleErrs is a measure of the number of errors for processing rules config.
	RuleErrs = monitoring.NewGauge(
		"mixer/config/rule_config_errors_total",
		"The number of errors encountered during processing of the rule configuration.",
	)

	// AdapterInfosTotal is a measure of the number of known adapters.
	AdapterInfosTotal = monitoring.NewGauge(
		"mixer/config/adapter_info_configs_total",
		"The number of known adapters in the current config.",
	)

	want = []metrics.Exported{
		{"mixer_config_adapter_info_configs_total", "LastValue", "The number of known adapters in the current config."},
		{"mixer_config_attributes_total", "LastValue", "The number of known attributes in the current config."},
		{"mixer_config_handler_configs_total", "LastValue", "The number of known handlers in the current config."},
		{"mixer_config_instance_config_errors_total", "LastValue", "The number of errors encountered during processing of the instance configuration."},
		{"mixer_config_instance_configs_total", "LastValue", "The number of known instances in the current config."},
		{"mixer_config_rule_config_errors_total", "LastValue", "The number of errors encountered during processing of the rule configuration."},
		{"mixer_config_rule_configs_total", "LastValue", "The number of known rules in the current config."},
	}
)

func init() {
	monitoring.MustRegister(
		AttributesTotal,
		HandlersTotal,
		InstancesTotal,
		InstanceErrs,
		RulesTotal,
		RuleErrs,
		AdapterInfosTotal,
	)

}
