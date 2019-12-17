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

package testing

import (
	v1 "k8s.io/api/core/v1"
	"knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/pkg/kmeta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapOption enables further configuration of a ConfigMap.
type ConfigMapOption func(*v1.ConfigMap)

// NewConfigMap creates a eew ConfigMap.
func NewConfigMap(name, namespace string, o ...ConfigMapOption) *v1.ConfigMap {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(cm)
	}
	return cm
}

func WithConfigMapLabels(labels map[string]string) ConfigMapOption {
	return func(cm *v1.ConfigMap) {
		cm.ObjectMeta.Labels = labels
	}
}

func WithConfigMapOwnerReference(ConfigMapPropagation *v1alpha1.ConfigMapPropagation) ConfigMapOption {
	return func(cm *v1.ConfigMap) {
		cm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			*kmeta.NewControllerRef(ConfigMapPropagation),
		}
	}
}

func WithConfigMapData(data map[string]string) ConfigMapOption {
	return func(cm *v1.ConfigMap) {
		cm.Data = data
	}
}
