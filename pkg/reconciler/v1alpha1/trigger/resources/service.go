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
	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NewService returns a K8s placeholder service for trigger 't'.
func NewService(t *eventingv1alpha1.Trigger) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    t.Namespace,
			GenerateName: fmt.Sprintf("trigger-%s-", t.Name),
			Labels:       ServiceLabels(t),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(t, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Trigger",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}
}
