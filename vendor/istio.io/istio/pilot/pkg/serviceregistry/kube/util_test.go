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

package kube

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestHasProxyIP(t *testing.T) {
	var tests = []struct {
		name      string
		addresses []v1.EndpointAddress
		proxyIP   string
		expected  bool
	}{
		{
			"has proxy ip",
			[]v1.EndpointAddress{{IP: "172.17.0.1"}, {IP: "172.17.0.2"}},
			"172.17.0.1",
			true,
		},
		{
			"has no proxy ip",
			[]v1.EndpointAddress{{IP: "172.17.0.1"}, {IP: "172.17.0.2"}},
			"172.17.0.100",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hasProxyIP(test.addresses, test.proxyIP)
			if test.expected != got {
				t.Errorf("Expected %v, but got %v", test.expected, got)
			}
		})
	}
}
