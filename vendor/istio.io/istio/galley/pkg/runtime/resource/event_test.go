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

package resource

import (
	"strings"
	"testing"
)

func TestEventKind_String(t *testing.T) {
	tests := map[EventKind]string{
		None:     "None",
		Added:    "Added",
		Updated:  "Updated",
		Deleted:  "Deleted",
		FullSync: "FullSync",
		55:       "<<Unknown EventKind 55>>",
	}

	for i, e := range tests {
		t.Run(e, func(t *testing.T) {
			a := i.String()
			if a != e {
				t.Fatalf("Mismatch: Actual=%v, Expected=%v", a, e)
			}
		})
	}
}

func TestEvent_String(t *testing.T) {
	tests := []struct {
		i   Event
		exp string
	}{
		{
			i:   Event{},
			exp: "[Event](None)",
		},
		{
			i:   Event{Kind: Added, Entry: Entry{ID: VersionedKey{Version: "foo", Key: Key{FullName: FullName{"fn"}}}}},
			exp: "[Event](Added: [VKey](:fn @foo))",
		},
		{
			i:   Event{Kind: Updated, Entry: Entry{ID: VersionedKey{Version: "foo", Key: Key{FullName: FullName{"fn"}}}}},
			exp: "[Event](Updated: [VKey](:fn @foo))",
		},
		{
			i:   Event{Kind: Deleted, Entry: Entry{ID: VersionedKey{Version: "foo", Key: Key{FullName: FullName{"fn"}}}}},
			exp: "[Event](Deleted: [VKey](:fn @foo))",
		},
		{
			i:   Event{Kind: FullSync},
			exp: "[Event](FullSync)",
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			actual := tc.i.String()
			if strings.TrimSpace(actual) != strings.TrimSpace(tc.exp) {
				t.Fatalf("Mismatch. got:%v, expected:%v", actual, tc.exp)
			}
		})
	}
}
