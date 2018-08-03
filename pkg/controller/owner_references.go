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

	channelsv1alpha "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	flowsv1alpha "github.com/knative/eventing/pkg/apis/flows/v1alpha1"
)

func kind(obj metav1.Object) schema.GroupVersionKind {
	switch obj.(type) {
	// Channels
	case *channelsv1alpha.Bus:
		return channelsv1alpha.SchemeGroupVersion.WithKind("Bus")
	case *channelsv1alpha.Channel:
		return channelsv1alpha.SchemeGroupVersion.WithKind("Channel")
	case *channelsv1alpha.ClusterBus:
		return channelsv1alpha.SchemeGroupVersion.WithKind("ClusterBus")

	// Feeds
	case *feedsv1alpha.ClusterEventType:
		return feedsv1alpha.SchemeGroupVersion.WithKind("ClusterEventType")
	case *feedsv1alpha.ClusterEventSource:
		return feedsv1alpha.SchemeGroupVersion.WithKind("ClusterEventSource")
	case *feedsv1alpha.EventType:
		return feedsv1alpha.SchemeGroupVersion.WithKind("EventType")
	case *feedsv1alpha.EventSource:
		return feedsv1alpha.SchemeGroupVersion.WithKind("EventSource")

	// Flows
	case *flowsv1alpha.Flow:
		return flowsv1alpha.SchemeGroupVersion.WithKind("Flow")

	default:
		panic(fmt.Sprintf("Unsupported object type %T", obj))
	}
}

// NewControllerRef creates an OwnerReference pointing to the given Resource.
func NewControllerRef(obj metav1.Object) *metav1.OwnerReference {
	blockOwnerDeletion := false
	ref := metav1.NewControllerRef(obj, kind(obj))
	ref.BlockOwnerDeletion = &blockOwnerDeletion
	return ref
}
