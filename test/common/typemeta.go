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

package common

import (
	"github.com/knative/eventing/test/base/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SubscriptionTypeMeta is the TypeMeta ref for Subscription.
var SubscriptionTypeMeta = EventingTypeMeta(resources.SubscriptionKind)

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

// CronJobSourceTypeMeta is the TypeMeta ref for CronJobSource.
var CronJobSourceTypeMeta = SourcesTypeMeta(resources.CronJobSourceKind)

// ContainerSourceTypeMeta is the TypeMeta ref for ContainerSource.
var ContainerSourceTypeMeta = SourcesTypeMeta(resources.ContainerSourceKind)

// ApiServerSourceTypeMeta is the TypeMeta ref for ApiServerSource.
var ApiServerSourceTypeMeta = SourcesTypeMeta(resources.ApiServerSourceKind)

// SourcesTypeMeta returns the TypeMeta ref for an eventing sources resource.
func SourcesTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.SourcesAPIVersion,
	}
}

// KafkaChannelTypeMeta is the TypeMeta ref for KafkaChannel.
var KafkaChannelTypeMeta = MessagingTypeMeta(resources.KafkaChannelKind)

// InMemoryChannelTypeMeta is the TypeMeta ref for InMemoryChannel.
var InMemoryChannelTypeMeta = MessagingTypeMeta(resources.InMemoryChannelKind)

// NatssChannelTypeMeta is the TypeMeta ref for NatssChannel.
var NatssChannelTypeMeta = MessagingTypeMeta(resources.NatssChannelKind)

// SequenceTypeMeta is the TypeMeta ref for Sequence.
var SequenceTypeMeta = MessagingTypeMeta(resources.SequenceKind)

// MessagingTypeMeta returns the TypeMeta ref for an eventing messaing resource.
func MessagingTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.MessagingAPIVersion,
	}
}
