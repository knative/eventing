/*
Copyright 2021 The Knative Authors

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

package kncloudevents

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSetAcceptReplyHeader(t *testing.T) {
	testCases := map[string]struct {
		additionalHeaders http.Header
		expectedHeaders   http.Header
	}{
		"No Prefer header": {
			additionalHeaders: map[string][]string{},
			expectedHeaders:   map[string][]string{"Prefer": {"reply"}},
		},
		"Reply exists": {
			additionalHeaders: map[string][]string{"Prefer": {"reply", "other"}},
			expectedHeaders:   map[string][]string{"Prefer": {"reply", "other"}},
		},
		"Reply exists with comma": {
			additionalHeaders: map[string][]string{"Prefer": {"reply, other"}},
			expectedHeaders:   map[string][]string{"Prefer": {"reply, other"}},
		},
		"Reply exists with equal": {
			additionalHeaders: map[string][]string{"Prefer": {"reply=, other"}},
			expectedHeaders:   map[string][]string{"Prefer": {"reply=, other"}},
		},
		"Reply exists with semicolon": {
			additionalHeaders: map[string][]string{"Prefer": {"reply; other"}},
			expectedHeaders:   map[string][]string{"Prefer": {"reply; other"}},
		},
		"Prefer header without reply": {
			additionalHeaders: map[string][]string{"Prefer": {"other"}},
			expectedHeaders:   map[string][]string{"Prefer": {"other", "reply"}},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			additionalHeaders := tc.additionalHeaders.Clone()
			SetAcceptReplyHeader(additionalHeaders)
			if diff := cmp.Diff(tc.expectedHeaders, additionalHeaders); diff != "" {
				t.Errorf("test receiver func -- bad headers (-want, +got): %s", diff)
			}
		})
	}
}
