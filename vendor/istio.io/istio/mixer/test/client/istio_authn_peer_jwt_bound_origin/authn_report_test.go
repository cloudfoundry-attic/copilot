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

package client_test

import (
	"fmt"
	"testing"

	"istio.io/istio/mixer/test/client/env"
	client_test "istio.io/istio/mixer/test/client/test_data"
)

// The Istio authn envoy config
// nolint
const authnConfig = `
- name: istio_authn
  config: {
    "policy": {
      "peers": [
        {
          "jwt": {
            "issuer": "issuer@foo.com",
            "jwks_uri": "http://localhost:8081/"
          }
        }
      ],
      "principal_binding": 1
    }
  }
`

const respExpected = "Origin authentication failed."

func TestAuthnCheckReportAttributesPeerJwtBoundToOrigin(t *testing.T) {
	s := env.NewTestSetup(env.CheckReportIstioAuthnAttributesTestPeerJwtBoundToOrigin, t)
	// In the Envoy config, principal_binding binds to origin
	s.SetFiltersBeforeMixer(client_test.JwtAuthConfig + authnConfig)
	// Disable the HotRestart of Envoy
	s.SetDisableHotRestart(true)

	env.SetStatsUpdateInterval(s.MfConfig(), 1)
	if err := s.SetUp(); err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}
	defer s.TearDown()

	url := fmt.Sprintf("http://localhost:%d/echo", s.Ports().ClientProxyPort)

	// Issues a GET echo request with 0 size body
	tag := "OKGet"

	headers := map[string]string{}
	headers["Authorization"] = "Bearer " + client_test.JwtTestToken

	// Principal is binded to origin, but no method specified in origin policy.
	// The request will be rejected by Istio authn filter.
	code, resp, err := env.HTTPGetWithHeaders(url, headers)
	if err != nil {
		t.Errorf("Failed in request %s: %v", tag, err)
	}
	// Verify that the http request is rejected
	if code != 401 {
		t.Errorf("Status code 401 is expected, got %d.", code)
	}
	if resp != respExpected {
		t.Errorf("Expected response: %s, got %s.", respExpected, resp)
	}
}
