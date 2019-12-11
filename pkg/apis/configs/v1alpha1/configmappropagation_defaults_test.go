/*
Copyright 2019 The Knative Authors

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

package v1alpha1

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"testing"
)

var (
	defaultNamespace = "knative-eventing"
	otherNamespace   = "testing-eventing"
	selector         = "knative.dev/testing: eventing"
)

func TestConfigMapPropagationDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  ConfigMapPropagation
		expected ConfigMapPropagation
	}{
		"nil spec": {
			initial:  ConfigMapPropagation{},
			expected: ConfigMapPropagation{Spec: ConfigMapPropagationSpec{OriginalNamespace: defaultNamespace}},
		},
		"original namespace empty": {
			initial:  ConfigMapPropagation{Spec: ConfigMapPropagationSpec{Selector: selector}},
			expected: ConfigMapPropagation{Spec: ConfigMapPropagationSpec{OriginalNamespace: defaultNamespace, Selector: selector}},
		},
		"with namespace": {
			initial:  ConfigMapPropagation{Spec: ConfigMapPropagationSpec{OriginalNamespace: otherNamespace}},
			expected: ConfigMapPropagation{Spec: ConfigMapPropagationSpec{OriginalNamespace: otherNamespace}},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.TODO())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
