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

package cache

import (
	"bytes"
	"context"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"istio.io/istio/security/pkg/nodeagent/model"
)

var (
	mockCertChain1st    = []string{"foo", "rootcert"}
	mockCertChainRemain = []string{"bar", "rootcert"}

	testResourceName = "default"
)

func TestGenerateSecret(t *testing.T) {
	fakeCACli := newMockCAClient()
	opt := Options{
		SecretTTL:        time.Minute,
		RotationInterval: 300 * time.Microsecond,
		EvictionDuration: 2 * time.Second,
	}
	sc := NewSecretCache(fakeCACli, notifyCb, opt)
	atomic.StoreUint32(&sc.skipTokenExpireCheck, 0)
	defer func() {
		sc.Close()
		atomic.StoreUint32(&sc.skipTokenExpireCheck, 1)
	}()

	proxyID := "proxy1-id"
	ctx := context.Background()
	gotSecret, err := sc.GenerateSecret(ctx, proxyID, testResourceName, "jwtToken1")
	if err != nil {
		t.Fatalf("Failed to get secrets: %v", err)
	}

	if got, want := gotSecret.CertificateChain, convertToBytes(mockCertChain1st); bytes.Compare(got, want) != 0 {
		t.Errorf("CertificateChain: got: %v, want: %v", got, want)
	}

	if got, want := sc.SecretExist(proxyID, testResourceName, "jwtToken1", gotSecret.Version), true; got != want {
		t.Errorf("SecretExist: got: %v, want: %v", got, want)
	}
	if got, want := sc.SecretExist(proxyID, testResourceName, "nonexisttoken", gotSecret.Version), false; got != want {
		t.Errorf("SecretExist: got: %v, want: %v", got, want)
	}

	gotSecretRoot, err := sc.GenerateSecret(ctx, proxyID, RootCertReqResourceName, "jwtToken1")
	if err != nil {
		t.Fatalf("Failed to get secrets: %v", err)
	}
	if got, want := gotSecretRoot.RootCert, []byte("rootcert"); bytes.Compare(got, want) != 0 {
		t.Errorf("CertificateChain: got: %v, want: %v", got, want)
	}

	if got, want := sc.SecretExist(proxyID, RootCertReqResourceName, "jwtToken1", gotSecretRoot.Version), true; got != want {
		t.Errorf("SecretExist: got: %v, want: %v", got, want)
	}
	if got, want := sc.SecretExist(proxyID, RootCertReqResourceName, "nonexisttoken", gotSecretRoot.Version), false; got != want {
		t.Errorf("SecretExist: got: %v, want: %v", got, want)
	}

	key := ConnKey{
		ProxyID:      proxyID,
		ResourceName: testResourceName,
	}
	cachedSecret, found := sc.secrets.Load(key)
	if !found {
		t.Errorf("Failed to find secret for proxy %q from secret store: %v", proxyID, err)
	}
	if !reflect.DeepEqual(*gotSecret, cachedSecret) {
		t.Errorf("Secret key: got %+v, want %+v", *gotSecret, cachedSecret)
	}

	// Try to get secret again using different jwt token, verify secret is re-generated.
	gotSecret, err = sc.GenerateSecret(ctx, proxyID, testResourceName, "newToken")
	if err != nil {
		t.Fatalf("Failed to get secrets: %v", err)
	}
	if got, want := gotSecret.CertificateChain, convertToBytes(mockCertChainRemain); bytes.Compare(got, want) != 0 {
		t.Errorf("CertificateChain: got: %v, want: %v", got, want)
	}

	// Wait until unused secrets are evicted.
	wait := 500 * time.Millisecond
	retries := 0
	for ; retries < 3; retries++ {
		time.Sleep(wait)
		if _, found := sc.secrets.Load(proxyID); found {
			// Retry after some sleep.
			wait *= 2
			continue
		}

		break
	}
	if retries == 3 {
		t.Errorf("Unused secrets failed to be evicted from cache")
	}
}

func TestRefreshSecret(t *testing.T) {
	fakeCACli := newMockCAClient()
	opt := Options{
		SecretTTL:        200 * time.Microsecond,
		RotationInterval: 200 * time.Microsecond,
		EvictionDuration: 10 * time.Second,
	}
	sc := NewSecretCache(fakeCACli, notifyCb, opt)
	atomic.StoreUint32(&sc.skipTokenExpireCheck, 0)
	defer func() {
		sc.Close()
		atomic.StoreUint32(&sc.skipTokenExpireCheck, 1)
	}()

	_, err := sc.GenerateSecret(context.Background(), "proxy1-id", testResourceName, "jwtToken1")
	if err != nil {
		t.Fatalf("Failed to get secrets: %v", err)
	}

	// Wait until key rotation job run to update cached secret.
	wait := 200 * time.Millisecond
	retries := 0
	for ; retries < 5; retries++ {
		time.Sleep(wait)
		if atomic.LoadUint64(&sc.secretChangedCount) == uint64(0) {
			// Retry after some sleep.
			wait *= 2
			continue
		}

		break
	}
	if retries == 5 {
		t.Errorf("Cached secret failed to get refreshed, %d", atomic.LoadUint64(&sc.secretChangedCount))
	}
}

func notifyCb(string, string, *model.SecretItem) error {
	return nil
}

type mockCAClient struct {
	signInvokeCount uint64
}

func newMockCAClient() *mockCAClient {
	cl := mockCAClient{}
	atomic.StoreUint64(&cl.signInvokeCount, 0)
	return &cl
}

func (c *mockCAClient) CSRSign(ctx context.Context, csrPEM []byte, subjectID string,
	certValidTTLInSec int64) ([]string /*PEM-encoded certificate chain*/, error) {
	atomic.AddUint64(&c.signInvokeCount, 1)

	if atomic.LoadUint64(&c.signInvokeCount) == 1 {
		return mockCertChain1st, nil
	}

	return mockCertChainRemain, nil
}

func convertToBytes(ss []string) []byte {
	res := []byte{}
	for _, s := range ss {
		res = append(res, []byte(s)...)
	}
	return res
}
