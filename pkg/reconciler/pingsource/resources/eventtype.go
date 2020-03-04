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
	"knative.dev/pkg/kmeta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/utils"
)

// MakeEventType creates the in-memory representation of the EventType for the specified PingSource.
func MakeEventType(src *v1alpha1.PingSource) *eventingv1alpha1.EventType {
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.GenerateFixedName(src, utils.ToDNS1123Subdomain(v1alpha1.PingSourceEventType)),
			Labels:    Labels(src.Name),
			Namespace: src.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(src),
			},
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   v1alpha1.PingSourceEventType,
			Source: v1alpha1.PingSourceSource(src.Namespace, src.Name),
			Broker: src.Spec.Sink.GetRef().Name,
		},
	}
}
