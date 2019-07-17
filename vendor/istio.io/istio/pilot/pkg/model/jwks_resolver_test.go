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

package model

import (
	"sync/atomic"
	"testing"
	"time"

	authn "istio.io/api/authentication/v1alpha1"
	"istio.io/istio/pilot/pkg/model/test"
)

func TestResolveJwksURIUsingOpenID(t *testing.T) {
	r := newJwksResolver(JwtPubKeyEvictionDuration, JwtPubKeyRefreshInterval)

	ms, err := test.StartNewServer()
	defer ms.Stop()
	if err != nil {
		t.Fatal("failed to start a mock server")
	}

	mockCertURL := ms.URL + "/oauth2/v3/certs"
	cases := []struct {
		in              string
		expectedJwksURI string
		expectedError   bool
	}{
		{
			in:              ms.URL,
			expectedJwksURI: mockCertURL,
		},
		{
			in:              ms.URL, // Send two same request, mock server is expected to hit only once because of the cache.
			expectedJwksURI: mockCertURL,
		},
		{
			in:            "http://xyz",
			expectedError: true,
		},
	}
	for _, c := range cases {
		jwksURI, err := r.resolveJwksURIUsingOpenID(c.in)
		if err != nil && !c.expectedError {
			t.Errorf("resolveJwksURIUsingOpenID(%+v): got error (%v)", c.in, err)
		} else if err == nil && c.expectedError {
			t.Errorf("resolveJwksURIUsingOpenID(%+v): expected error, got no error", c.in)
		} else if c.expectedJwksURI != jwksURI {
			t.Errorf("resolveJwksURIUsingOpenID(%+v): expected (%s), got (%s)",
				c.in, c.expectedJwksURI, jwksURI)
		}
	}

	// Verify mock openID discovery http://localhost:9999/.well-known/openid-configuration was only called once because of the cache.
	if got, want := ms.OpenIDHitNum, uint64(1); got != want {
		t.Errorf("Mock OpenID discovery Hit number => expected %d but got %d", want, got)
	}
}

func TestSetAuthenticationPolicyJwksURIs(t *testing.T) {
	r := newJwksResolver(JwtPubKeyEvictionDuration, JwtPubKeyRefreshInterval)

	ms, err := test.StartNewServer()
	defer ms.Stop()
	if err != nil {
		t.Fatal("failed to start a mock server")
	}

	mockCertURL := ms.URL + "/oauth2/v3/certs"

	authNPolicies := map[string]*authn.Policy{
		"one": {
			Targets: []*authn.TargetSelector{{
				Name: "one",
				Ports: []*authn.PortSelector{
					{
						Port: &authn.PortSelector_Number{
							Number: 80,
						},
					},
				},
			}},
			Origins: []*authn.OriginAuthenticationMethod{
				{
					Jwt: &authn.Jwt{
						Issuer: ms.URL,
					},
				},
			},
			PrincipalBinding: authn.PrincipalBinding_USE_ORIGIN,
		},
		"two": {
			Targets: []*authn.TargetSelector{{
				Name: "two",
				Ports: []*authn.PortSelector{
					{
						Port: &authn.PortSelector_Number{
							Number: 80,
						},
					},
				},
			}},
			Origins: []*authn.OriginAuthenticationMethod{
				{
					Jwt: &authn.Jwt{
						Issuer:  "http://abc",
						JwksUri: "http://xyz",
					},
				},
			},
			PrincipalBinding: authn.PrincipalBinding_USE_ORIGIN,
		},
	}

	cases := []struct {
		in       *authn.Policy
		expected string
	}{
		{
			in:       authNPolicies["one"],
			expected: mockCertURL,
		},
		{
			in:       authNPolicies["two"],
			expected: "http://xyz",
		},
	}
	for _, c := range cases {
		_ = r.SetAuthenticationPolicyJwksURIs(c.in)
		got := c.in.GetOrigins()[0].GetJwt().JwksUri
		if want := c.expected; got != want {
			t.Errorf("setAuthenticationPolicyJwksURIs(%+v): expected (%s), got (%s)", c.in, c.expected, c.in)
		}
	}
}

func TestGetPublicKey(t *testing.T) {
	r := newJwksResolver(JwtPubKeyEvictionDuration, JwtPubKeyRefreshInterval)
	defer r.Close()

	ms, err := test.StartNewServer()
	defer ms.Stop()
	if err != nil {
		t.Fatal("failed to start a mock server")
	}

	mockCertURL := ms.URL + "/oauth2/v3/certs"

	cases := []struct {
		in                string
		expectedJwtPubkey string
	}{
		{
			in:                mockCertURL,
			expectedJwtPubkey: test.JwtPubKey1,
		},
		{
			in:                mockCertURL, // Send two same request, mock server is expected to hit only once because of the cache.
			expectedJwtPubkey: test.JwtPubKey1,
		},
	}
	for _, c := range cases {
		pk, err := r.GetPublicKey(c.in)
		if err != nil {
			t.Errorf("GetPublicKey(%+v) fails: expected no error, got (%v)", c.in, err)
		}
		if c.expectedJwtPubkey != pk {
			t.Errorf("GetPublicKey(%+v): expected (%s), got (%s)", c.in, c.expectedJwtPubkey, pk)
		}
	}

	// Verify mock server http://localhost:9999/oauth2/v3/certs was only called once because of the cache.
	if got, want := ms.PubKeyHitNum, uint64(1); got != want {
		t.Errorf("Mock server Hit number => expected %d but got %d", want, got)
	}
}

func TestJwtPubKeyEvictionForNotUsed(t *testing.T) {
	r := newJwksResolver(100*time.Millisecond /*EvictionDuration*/, 2*time.Millisecond /*RefreshInterval*/)
	defer r.Close()

	ms := startMockServer(t)
	defer ms.Stop()

	// Mock server returns JwtPubKey2 for later calls.
	// Verify the refresher has run and got new key from mock server.
	verifyKeyRefresh(t, r, ms, test.JwtPubKey2)

	// Wait until unused keys are evicted.
	mockCertURL := ms.URL + "/oauth2/v3/certs"
	retries := 0
	for ; retries < 3; retries++ {
		time.Sleep(time.Second)
		// Verify the public key is evicted.
		if _, found := r.keyEntries.Load(mockCertURL); found {
			// Retry after some sleep.
			continue
		}
		break
	}
	if retries == 3 {
		t.Errorf("Unused keys failed to be evicted")
	}
}

func TestJwtPubKeyEvictionForNotRefreshed(t *testing.T) {
	r := newJwksResolver(2*time.Second /*EvictionDuration*/, 100*time.Millisecond /*RefreshInterval*/)
	defer r.Close()

	ms := startMockServer(t)
	defer ms.Stop()

	// Configures the mock server to return error after the first request.
	ms.ReturnErrorAfterFirstNumHits = 1

	mockCertURL := ms.URL + "/oauth2/v3/certs"

	// Keep getting the public key to change the lastUsedTime of the public key.
	done := make(chan struct{})
	go func() {
		c := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-done:
				return
			case <-c.C:
				_, _ = r.GetPublicKey(mockCertURL)
			}
		}
	}()
	defer func() {
		done <- struct{}{}
	}()

	pk, err := r.GetPublicKey(mockCertURL)
	if err != nil {
		t.Fatalf("GetPublicKey(%+v) fails: expected no error, got (%v)", mockCertURL, err)
	}
	// Mock server returns JwtPubKey1 for first call.
	if test.JwtPubKey1 != pk {
		t.Fatalf("GetPublicKey(%+v): expected (%s), got (%s)", mockCertURL, test.JwtPubKey1, pk)
	}

	// Verify the cached public key is removed after failed to refresh longer than the eviction duration.
	time.Sleep(5 * time.Second)
	_, err = r.GetPublicKey(mockCertURL)
	if err == nil {
		t.Errorf("GetPublicKey(%+v) fails: expected error, got no error", mockCertURL)
	}
}

func TestJwtPubKeyLastRefreshedTime(t *testing.T) {
	r := newJwksResolver(JwtPubKeyEvictionDuration, 2*time.Millisecond /*RefreshInterval*/)
	defer r.Close()

	ms := startMockServer(t)
	defer ms.Stop()

	// Mock server returns JwtPubKey2 for later calls.
	// Verify the refresher has run and got new key from mock server.
	verifyKeyRefresh(t, r, ms, test.JwtPubKey2)

	// The lastRefreshedTime should change for each successful refresh.
	verifyKeyLastRefreshedTime(t, r, ms, true /* wantChanged */)
}

func TestJwtPubKeyRefreshWithNetworkError(t *testing.T) {
	r := newJwksResolver(JwtPubKeyEvictionDuration, time.Second /*RefreshInterval*/)
	defer r.Close()

	ms := startMockServer(t)
	defer ms.Stop()

	// Configures the mock server to return error after the first request.
	ms.ReturnErrorAfterFirstNumHits = 1

	// The refresh job should continue using the previously fetched public key (JwtPubKey1).
	verifyKeyRefresh(t, r, ms, test.JwtPubKey1)

	// The lastRefreshedTime should not change the refresh failed due to network error.
	verifyKeyLastRefreshedTime(t, r, ms, false /* wantChanged */)
}

func startMockServer(t *testing.T) *test.MockOpenIDDiscoveryServer {
	t.Helper()

	ms, err := test.StartNewServer()
	if err != nil {
		t.Fatal("failed to start a mock server")
	}
	return ms
}

func verifyKeyRefresh(t *testing.T, r *jwksResolver, ms *test.MockOpenIDDiscoveryServer, expectedJwtPubkey string) {
	t.Helper()
	mockCertURL := ms.URL + "/oauth2/v3/certs"

	pk, err := r.GetPublicKey(mockCertURL)
	if err != nil {
		t.Fatalf("GetPublicKey(%+v) fails: expected no error, got (%v)", mockCertURL, err)
	}
	// Mock server returns JwtPubKey1 for first call.
	if test.JwtPubKey1 != pk {
		t.Fatalf("GetPublicKey(%+v): expected (%s), got (%s)", mockCertURL, test.JwtPubKey1, pk)
	}

	// Wait until refresh job at least finished once.
	retries := 0
	for ; retries < 20; retries++ {
		time.Sleep(time.Second)
		// Make sure refresh job has run and detect change or refresh happened.
		if atomic.LoadUint64(&r.refreshJobKeyChangedCount) > 0 || atomic.LoadUint64(&r.refreshJobFetchFailedCount) > 0 {
			break
		}
	}
	if retries == 20 {
		t.Fatalf("Refresher failed to run")
	}

	pk, err = r.GetPublicKey(mockCertURL)
	if err != nil {
		t.Fatalf("GetPublicKey(%+v) fails: expected no error, got (%v)", mockCertURL, err)
	}
	if expectedJwtPubkey != pk {
		t.Fatalf("GetPublicKey(%+v): expected (%s), got (%s)", mockCertURL, expectedJwtPubkey, pk)
	}
}

func verifyKeyLastRefreshedTime(t *testing.T, r *jwksResolver, ms *test.MockOpenIDDiscoveryServer, wantChanged bool) {
	t.Helper()
	mockCertURL := ms.URL + "/oauth2/v3/certs"

	e, found := r.keyEntries.Load(mockCertURL)
	if !found {
		t.Fatalf("No cached public key for %s", mockCertURL)
	}
	oldRefreshedTime := e.(jwtPubKeyEntry).lastRefreshedTime

	time.Sleep(200 * time.Millisecond)

	e, found = r.keyEntries.Load(mockCertURL)
	if !found {
		t.Fatalf("No cached public key for %s", mockCertURL)
	}
	newRefreshedTime := e.(jwtPubKeyEntry).lastRefreshedTime

	if actualChanged := oldRefreshedTime != newRefreshedTime; actualChanged != wantChanged {
		t.Errorf("Want changed: %t but got %t", wantChanged, actualChanged)
	}
}
