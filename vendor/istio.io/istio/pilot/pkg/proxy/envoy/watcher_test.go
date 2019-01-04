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
	"bytes"
	"context"
	"crypto/sha256"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/howeyc/fsnotify"

	"istio.io/istio/pilot/pkg/model"
)

type TestAgent struct {
	configCh chan interface{}
}

func (ta *TestAgent) ConfigCh() chan<- interface{} {
	return ta.configCh
}

func (ta *TestAgent) Run(ctx context.Context) {
	<-ctx.Done()
}

func TestRunSendConfig(t *testing.T) {
	agent := &TestAgent{
		configCh: make(chan interface{}),
	}
	watcher := NewWatcher([]CertSource{{Directory: "random"}}, agent.ConfigCh())
	ctx, cancel := context.WithCancel(context.Background())

	// watcher starts agent and schedules a config update
	go watcher.Run(ctx)

	select {
	case <-agent.configCh:
		// expected
		cancel()
	case <-time.After(time.Second):
		t.Errorf("The callback is not called within time limit " + time.Now().String())
		cancel()
	}
}

func TestWatchCerts_Multiple(t *testing.T) {

	lock := sync.Mutex{}
	called := 0

	callback := func() {
		lock.Lock()
		defer lock.Unlock()
		called++
	}

	maxDelay := 500 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	wch := make(chan *fsnotify.FileEvent, 10)

	go watchFileEvents(ctx, wch, maxDelay, callback)

	// fire off multiple events
	wch <- &fsnotify.FileEvent{Name: "f1"}
	wch <- &fsnotify.FileEvent{Name: "f2"}
	wch <- &fsnotify.FileEvent{Name: "f3"}

	// sleep for less than maxDelay
	time.Sleep(maxDelay / 2)

	// Expect no events to be delivered within maxDelay.
	lock.Lock()
	if called != 0 {
		t.Fatalf("Called %d times, want 0", called)
	}
	lock.Unlock()

	// wait for quiet period
	time.Sleep(maxDelay)

	// Expect exactly 1 event to be delivered.
	lock.Lock()
	defer lock.Unlock()
	if called != 1 {
		t.Fatalf("Called %d times, want 1", called)
	}

	cancel()
}

func TestWatchCerts(t *testing.T) {
	name, err := ioutil.TempDir(os.TempDir(), "certs")
	if err != nil {
		t.Errorf("failed to create a temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(name); err != nil {
			t.Errorf("failed to remove temp dir: %v", err)
		}
	}()

	called := make(chan bool)
	callbackFunc := func() {
		called <- true
	}

	ctx, cancel := context.WithCancel(context.Background())

	go watchCerts(ctx, []string{name}, watchFileEvents, 50*time.Millisecond, callbackFunc)

	// sleep one second to make sure the watcher is set up before change is made
	time.Sleep(time.Second)

	// make a change to the watched dir
	if _, err := ioutil.TempFile(name, "test.file"); err != nil {
		t.Errorf("failed to create a temp file in testdata/certs: %v", err)
	}

	select {
	case <-called:
		// expected
		cancel()
	case <-time.After(time.Second):
		t.Errorf("The callback is not called within time limit " + time.Now().String())
		cancel()
	}

	// should terminate immediately
	go watchCerts(ctx, nil, watchFileEvents, 50*time.Millisecond, callbackFunc)
}

func TestGenerateCertHash(t *testing.T) {
	name, err := ioutil.TempDir(os.TempDir(), "certs")
	if err != nil {
		t.Errorf("failed to create a temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(name); err != nil {
			t.Errorf("failed to remove temp dir: %v", err)
		}
	}()

	h := sha256.New()
	authFiles := []string{model.CertChainFilename, model.KeyFilename, model.RootCertFilename}
	for _, file := range authFiles {
		content := []byte(file)
		if err := ioutil.WriteFile(path.Join(name, file), content, 0644); err != nil {
			t.Errorf("failed to write file %s (error %v)", file, err)
		}
		if _, err := h.Write(content); err != nil {
			t.Errorf("failed to write hash (error %v)", err)
		}
	}
	expectedHash := h.Sum(nil)

	h2 := sha256.New()
	generateCertHash(h2, name, append(authFiles, "missing-file"))
	actualHash := h2.Sum(nil)
	if !bytes.Equal(actualHash, expectedHash) {
		t.Errorf("Actual hash value (%v) is different than the expected hash value (%v)", actualHash, expectedHash)
	}

	generateCertHash(h2, "", nil)
	emptyHash := h2.Sum(nil)
	if !bytes.Equal(emptyHash, expectedHash) {
		t.Error("hash should not be affected by empty directory")
	}
}
