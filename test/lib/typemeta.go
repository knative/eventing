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

package lib

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/test/lib/resources"
)

// SubscriptionTypeMeta is the TypeMeta ref for Subscription.
var SubscriptionTypeMeta = MessagingTypeMeta(resources.SubscriptionKind)

// BrokerTypeMeta is the TypeMeta ref for Broker.
var BrokerTypeMeta = EventingTypeMeta(resources.BrokerKind)

// TriggerTypeMeta is the TypeMeta ref for Trigger.
var TriggerTypeMeta = EventingTypeMeta(resources.TriggerKind)

// EventingTypeMeta returns the TypeMeta ref for an eventing resource.
func EventingTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.EventingAPIVersion,
	}
}

// ChannelTypeMeta is the TypeMeta ref for Channel.
var ChannelTypeMeta = MessagingTypeMeta(resources.ChannelKind)

// SequenceTypeMeta is the TypeMeta ref for Sequence.
var SequenceTypeMeta = MessagingTypeMeta(resources.SequenceKind)

// ParallelTypeMeta is the TypeMeta ref for Parallel.
var ParallelTypeMeta = MessagingTypeMeta(resources.ParallelKind)

// MessagingTypeMeta returns the TypeMeta ref for an eventing messaging resource.
func MessagingTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.MessagingAPIVersion,
	}
}

// FlowsParallelTypeMeta is the TypeMeta ref for Parallel (in flows.knative.dev).
var FlowsParallelTypeMeta = FlowsTypeMeta(resources.FlowsParallelKind)

// FlowsSequenceTypeMeta is the TypeMeta ref for Sequence (in flows.knative.dev).
var FlowsSequenceTypeMeta = FlowsTypeMeta(resources.FlowsSequenceKind)

// FlowsTypeMeta returns the TypeMeta ref for an eventing messaging resource.
func FlowsTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.FlowsAPIVersion,
	}
}
