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
	"knative.dev/pkg/kmeta"
)

func TestMakeConfigMap(t *testing.T) {
	testCases := []struct {
		name                 string
		original             *corev1.ConfigMap
		configmappropagation *configsv1alpha1.ConfigMapPropagation
		wantName             string
		wantLabels           map[string]string
	}{
		{
			name: "normal",
			original: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "original-config-map",
				},
			},
			configmappropagation: &configsv1alpha1.ConfigMapPropagation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cmp",
					Namespace: "testing",
				},
			},
			wantName: "cmp-original-config-map",
			wantLabels: map[string]string{
				PropagationLabelKey: PropagationLabelValueCopy,
				CopyLabelKey:        "testing-original-config-map",
			},
		}, {
			name: "long name",
			original: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "original-config-map-very-very-very-very-very-very-very-long-name",
				},
			},
			configmappropagation: &configsv1alpha1.ConfigMapPropagation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cmp",
					Namespace: "testing",
				},
			},
			wantName: kmeta.ChildName("cmp", "-original-config-map-very-very-very-very-very-very-very-long-name"),
			wantLabels: map[string]string{
				PropagationLabelKey: PropagationLabelValueCopy,
				CopyLabelKey:        kmeta.ChildName("testing", "-original-config-map-very-very-very-very-very-very-very-long-name"),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configmap := MakeConfigMap(ConfigMapArgs{
				tc.original, tc.configmappropagation,
			})
			// If name and namespace are correct.
			if name := configmap.Name; name != tc.wantName {
				t.Errorf("want %v, got %v", tc.wantName, name)
			}

			// name cannot be longer than 63 characters.
			if len(configmap.Name) > 63 {
				t.Errorf("copy configmap name %q is longer than 63 characters", configmap.Name)
			}

			if ns := configmap.Namespace; ns != tc.configmappropagation.Namespace {
				t.Errorf("want %v, got %v", tc.configmappropagation.Namespace, ns)
			}

			// If OwnerReferences is correct.
			if !metav1.IsControlledBy(configmap, tc.configmappropagation) {
				t.Errorf("Expected configmap to be controlled by the configmappropagation")
			}

			// If labels are correct.
			if labels := configmap.Labels; !reflect.DeepEqual(labels, tc.wantLabels) {
				t.Errorf("want %v, got %q", tc.wantLabels, labels)
			}

			// Label values cannot be longer than 63 characters.
			for _, v := range configmap.Labels {
				if len(v) > 63 {
					t.Errorf("copy configmap label value %q is longer than 63 characters", v)
				}
			}

		})
	}
}
