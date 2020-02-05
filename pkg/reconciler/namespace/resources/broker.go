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

package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
)

// MakeBroker creates a default Broker object for Namespace 'namespace'.
func MakeBroker(ns *corev1.Namespace) *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ns.GetObjectMeta(), schema.GroupVersionKind{
					Group:   corev1.SchemeGroupVersion.Group,
					Version: corev1.SchemeGroupVersion.Version,
					Kind:    "Namespace",
				}),
			},
			Namespace: ns.Name,
			Name:      DefaultBrokerName,
			Labels:    OwnedLabels(),
		},
	}
}
