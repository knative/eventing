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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	namespace       = "testing-eventing"
	defaultSelector = metav1.LabelSelector{}
	otherSelector   = metav1.LabelSelector{MatchLabels: map[string]string{"testing": "testing"}}
)

func TestConfigMapPropagationDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  ConfigMapPropagation
		expected ConfigMapPropagation
	}{
		"nil spec": {
			initial:  ConfigMapPropagation{},
			expected: ConfigMapPropagation{Spec: ConfigMapPropagationSpec{Selector: &defaultSelector}},
		},
		"selector empty": {
			initial:  ConfigMapPropagation{Spec: ConfigMapPropagationSpec{OriginalNamespace: namespace}},
			expected: ConfigMapPropagation{Spec: ConfigMapPropagationSpec{OriginalNamespace: namespace, Selector: &defaultSelector}},
		},
		"with selector": {
			initial:  ConfigMapPropagation{Spec: ConfigMapPropagationSpec{Selector: &otherSelector}},
			expected: ConfigMapPropagation{Spec: ConfigMapPropagationSpec{Selector: &otherSelector}},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.Background())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Errorf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
