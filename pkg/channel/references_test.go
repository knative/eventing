/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package channel

import (
	"fmt"
	"testing"

	_ "knative.dev/pkg/system/testing"
)

const (
	referencesTestNamespace   = "test-namespace"
	referencesTestChannelName = "test-channel"
)

func TestChannelReference_String(t *testing.T) {
	ref := ChannelReference{
		Name:      referencesTestChannelName,
		Namespace: referencesTestNamespace,
	}
	expected := fmt.Sprintf("%s/%s", referencesTestNamespace, referencesTestChannelName)
	actual := ref.String()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "ChannelReference", expected, actual)
	}
}

func TestParseChannelFromHost(t *testing.T) {
	testCases := map[string]struct {
		host               string
		wantErr            bool
		expectedChannelRef ChannelReference
	}{
		"host based": {
			host:    "test-channel.test-namespace.svc.cluster.local",
			wantErr: false,
			expectedChannelRef: ChannelReference{
				Namespace: "test-namespace",
				Name:      "test-channel",
			},
		},
		"bad host format should return error": {
			host:    "test-channel",
			wantErr: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c, err := ParseChannelFromHost(tc.host)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error but didn't get one")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			if tc.expectedChannelRef.Namespace != c.Namespace {
				t.Fatalf("expected Channel Namespace %s doesn't match actual Namespace %s", tc.expectedChannelRef.Namespace, c.Namespace)
			}

			if tc.expectedChannelRef.Name != c.Name {
				t.Fatalf("expected Channel Name %s doesn't match actual Name %s", tc.expectedChannelRef.Name, c.Name)
			}
		})
	}
}

func TestParseChannelFromPath(t *testing.T) {
	testCases := map[string]struct {
		path               string
		wantErr            bool
		expectedChannelRef ChannelReference
	}{
		"path based": {
			path:    "/new-namespace/new-channel/",
			wantErr: false,
			expectedChannelRef: ChannelReference{
				Namespace: "new-namespace",
				Name:      "new-channel",
			},
		},

		"bad path format should return error": {
			path:    "/something/",
			wantErr: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c, err := ParseChannelFromPath(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error but didn't get one")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			if tc.expectedChannelRef.Namespace != c.Namespace {
				t.Fatalf("expected Channel Namespace %s doesn't match actual Namespace %s", tc.expectedChannelRef.Namespace, c.Namespace)
			}

			if tc.expectedChannelRef.Name != c.Name {
				t.Fatalf("expected Channel Name %s doesn't match actual Name %s", tc.expectedChannelRef.Name, c.Name)
			}
		})
	}
}
