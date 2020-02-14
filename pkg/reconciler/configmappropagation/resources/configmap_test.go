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

package resources

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
)

func TestMakeConfigMap(t *testing.T) {
	testCases := map[string]struct {
		original             *corev1.ConfigMap
		configmappropagation *configsv1alpha1.ConfigMapPropagation
	}{
		"general configmap": {
			original: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "original-config-map",
					Namespace: "system",
				},
			},
			configmappropagation: &configsv1alpha1.ConfigMapPropagation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cmp",
					Namespace: "default",
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			configmap := MakeConfigMap(ConfigMapArgs{
				tc.original, tc.configmappropagation,
			})
			// If name and namespace are correct.
			if name := configmap.Name; name != "cmp-original-config-map" {
				t.Errorf("want %v, got %v", "cmp-original-config-map", name)
			}

			if ns := configmap.Namespace; ns != tc.configmappropagation.Namespace {
				t.Errorf("want %v, got %v", tc.configmappropagation.Namespace, ns)
			}

			// If OwnerReferences is correct.
			if !metav1.IsControlledBy(configmap, tc.configmappropagation) {
				t.Errorf("Expected configmap to be controlled by the configmappropagation")
			}

			// If labels are correct.
			expectedLabels := map[string]string{
				PropagationLabelKey: PropagationLabelValueCopy,
				CopyLabelKey:        "system-original-config-map",
			}
			if labels := configmap.Labels; !reflect.DeepEqual(labels, expectedLabels) {
				t.Errorf("want %v, got %q", expectedLabels, labels)
			}
		})
	}
}
