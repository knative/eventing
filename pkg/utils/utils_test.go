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

package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGetDomainName(t *testing.T) {
	tests := []struct {
		name       string
		resolvConf string
		want       string
	}{
		{
			name: "all good",
			resolvConf: `
nameserver 1.1.1.1
search default.svc.abc.com svc.abc.com abc.com
options ndots:5
`,
			want: "abc.com",
		},
		{
			name: "missing search line",
			resolvConf: `
nameserver 1.1.1.1
options ndots:5
`,
			want: defaultDomainName,
		},
		{
			name: "non k8s resolv.conf format",
			resolvConf: `
nameserver 1.1.1.1
search  abc.com xyz.com
options ndots:5
`,
			want: defaultDomainName,
		},
	}
	for _, tt := range tests {
		got := getClusterDomainName(strings.NewReader(tt.resolvConf))
		if got != tt.want {
			t.Errorf("Test %s failed expected: %s but got: %s", tt.name, tt.want, got)
		}
	}
}

func TestObjectRef(t *testing.T) {
	testCases := map[string]struct {
		obj metav1.Object
		gvk schema.GroupVersionKind
	}{
		"Service": {
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "my-ns",
					Name:      "my-name",
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Service",
			},
		},
		"Broker": {
			obj: &v1alpha1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "broker-ns",
					Name:      "my-broker",
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "eventing.knative.dev",
				Version: "v1alpha1",
				Kind:    "Broker",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			or := ObjectRef(tc.obj, tc.gvk)

			expectedApiVersion := fmt.Sprintf("%s/%s", tc.gvk.Group, tc.gvk.Version)
			// Special case for v1.
			if tc.gvk.Group == "" {
				expectedApiVersion = tc.gvk.Version
			}

			if api, _ := tc.gvk.ToAPIVersionAndKind(); api != expectedApiVersion {
				t.Errorf("Expected APIVersion %q, actually %q", expectedApiVersion, api)
			}
			if kind := or.Kind; kind != tc.gvk.Kind {
				t.Errorf("Expected kind %q, actually %q", tc.gvk.Kind, kind)
			}
			if ns := or.Namespace; ns != tc.obj.GetNamespace() {
				t.Errorf("Expected namespace %q, actually %q", tc.obj.GetNamespace(), ns)
			}
			if n := or.Name; n != tc.obj.GetName() {
				t.Errorf("Expected name %q, actually %q", tc.obj.GetName(), n)
			}
		})
	}
}

func TestToDNS1123Subdomain(t *testing.T) {
	testCases := map[string]struct {
		name     string
		expected string
	}{
		"short": {
			name:     "abc",
			expected: "abc",
		},
		"too long": {
			name:     strings.Repeat("a", 300),
			expected: strings.Repeat("a", 243),
		},
		"surrounded by dashes": {
			name:     "-foo-",
			expected: "foo",
		},
		"illegal characters": {
			name:     "a$b",
			expected: "ab",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			a := ToDNS1123Subdomain(tc.name)
			if a != tc.expected {
				t.Errorf("Expected %q, actually %q", tc.expected, a)
			}
		})
	}
}
