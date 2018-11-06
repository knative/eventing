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

package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	eventingv1alpha "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

func kind(obj metav1.Object) schema.GroupVersionKind {
	switch obj.(type) {

	// Eventing
	case *eventingv1alpha.Channel:
		return eventingv1alpha.SchemeGroupVersion.WithKind("Channel")
	case *eventingv1alpha.ClusterChannelProvisioner:
		return eventingv1alpha.SchemeGroupVersion.WithKind("ClusterChannelProvisioner")
	case *eventingv1alpha.Subscription:
		return eventingv1alpha.SchemeGroupVersion.WithKind("Subscription")

	default:
		panic(fmt.Sprintf("Unsupported object type %T", obj))
	}
}

// NewControllerRef creates an OwnerReference pointing to the given Resource.
func NewControllerRef(obj metav1.Object, blockOwnerDeletion bool) *metav1.OwnerReference {
	ref := metav1.NewControllerRef(obj, kind(obj))
	ref.BlockOwnerDeletion = &blockOwnerDeletion
	return ref
}
