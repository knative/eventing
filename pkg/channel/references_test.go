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
	"net/url"
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

func TestParseChannel(t *testing.T) {
	testCases := map[string]struct {
		url                url.URL
		wantErr            bool
		expectedChannelRef ChannelReference
	}{
		"host based": {
			url: url.URL{
				Host: "test-channel.test-namespace.svc.cluster.local",
				Path: "/",
			},
			wantErr: false,
			expectedChannelRef: ChannelReference{
				Namespace: "test-namespace",
				Name:      "test-channel",
			},
		},
		"path based": {
			url: url.URL{
				Host: "test-channel.test-namespace.svc.cluster.local",
				Path: "/new-namespace/new-channel/",
			},
			wantErr: false,
			expectedChannelRef: ChannelReference{
				Namespace: "new-namespace",
				Name:      "new-channel",
			},
		},

		"malformed URL should return error": {
			url:     url.URL{},
			wantErr: true,
		},

		"bad host format should return error": {
			url: url.URL{
				Host: "test-channel",
				Path: "/",
			},
			wantErr: true,
		},

		"bad path format should return error": {
			url: url.URL{
				Host: "dispatcher.svc.cluster.local",
				Path: "/something/",
			},
			wantErr: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c, err := ParseChannel(tc.url.String())
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
