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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/pkg/system"
)

// MakeConfigMapPropagation creates a default ConfigMapPropagation object for Namespace 'namespace'.
func MakeConfigMapPropagation(namespace string) *v1alpha1.ConfigMapPropagation {
	return &v1alpha1.ConfigMapPropagation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      DefaultConfigMapPropagationName,
		},
		Spec: v1alpha1.ConfigMapPropagationSpec{
			OriginalNamespace: system.Namespace(),
			Selector:          &metav1.LabelSelector{MatchLabels: ConfigMapPropagationOwnedLabels()},
		},
	}
}
