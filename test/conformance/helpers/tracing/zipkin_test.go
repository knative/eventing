/*
Copyright 2020 The Knative Authors

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

package tracing

import "testing"

func TestParseClusterDomainFromHostname(t *testing.T) {
	testCases := map[string]struct {
		hostname              string
		expectedClusterDomain string
		expectErr             bool
	}{
		"too few dots": {
			hostname:  "foo.bar",
			expectErr: true,
		},
		"not svc": {
			hostname:  "foo.bar.baz.qux.fin",
			expectErr: true,
		},
		"cluster local": {
			hostname:              "zipkin.knative-eventing.svc.cluster.local",
			expectedClusterDomain: "cluster.local",
		},
		"KinD cluster domain": {
			hostname:              "zipkin.knative-eventing.svc.c308965081.local",
			expectedClusterDomain: "c308965081.local",
		},
		"Empty domain": {
			hostname: "zipkin.knative-eventing.svc",
		},
		"cluster local with port": {
			hostname:              "zipkin.knative-eventing.svc.cluster.local:9411",
			expectedClusterDomain: "cluster.local",
		},
		"KinD cluster domain with port": {
			hostname:              "zipkin.knative-eventing.svc.c308965081.local:9411",
			expectedClusterDomain: "c308965081.local",
		},
		"Empty domain with port": {
			hostname: "zipkin.knative-eventing.svc:9411",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			d, err := parseClusterDomainFromHostname(tc.hostname)
			if err != nil {
				if !tc.expectErr {
					t.Fatal("Unexpected error:", err)
				}
			}
			if d != tc.expectedClusterDomain {
				t.Fatalf("Unexpected cluster domain. want %q, got %q", tc.expectedClusterDomain, d)
			}
		})
	}
}
