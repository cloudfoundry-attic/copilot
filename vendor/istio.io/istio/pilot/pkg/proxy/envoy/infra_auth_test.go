// Copyright 2017 Istio Authors
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

package envoy

import (
	"strings"
	"testing"

	"istio.io/istio/pkg/spiffe"
)

const (
	expMixerSAN string = "spiffe://cluster.local/ns/istio-system/sa/istio-mixer-service-account"
	expPilotSAN string = "spiffe://cluster.local/ns/istio-system/sa/istio-pilot-service-account"
)

func TestGetMixerSAN(t *testing.T) {
	spiffe.SetTrustDomain("cluster.local")
	mixerSANs := GetSAN("istio-system", MixerSvcAccName)
	if strings.Compare(mixerSANs, expMixerSAN) != 0 {
		t.Errorf("GetMixerSAN() => expected %#v but got %#v", expMixerSAN, mixerSANs[0])
	}
}

func TestGetPilotSAN(t *testing.T) {
	spiffe.SetTrustDomain("cluster.local")
	pilotSANs := GetSAN("istio-system", PilotSvcAccName)
	if strings.Compare(pilotSANs, expPilotSAN) != 0 {
		t.Errorf("GetPilotSAN() => expected %#v but got %#v", expPilotSAN, pilotSANs[0])
	}
}
