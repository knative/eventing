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
	"github.com/knative/eventing/test/base"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChannelTypeMeta is the TypeMeta ref for Channel.
var ChannelTypeMeta = EventingTypeMeta(base.ChannelKind)

// SubscriptionTypeMeta is the TypeMeta ref for Subscription.
var SubscriptionTypeMeta = EventingTypeMeta(base.SubscriptionKind)

// BrokerTypeMeta is the TypeMeta ref for Broker.
var BrokerTypeMeta = EventingTypeMeta(base.BrokerKind)

// TriggerTypeMeta is the TypeMeta ref for Trigger.
var TriggerTypeMeta = EventingTypeMeta(base.TriggerKind)

// EventingTypeMeta returns the TypeMeta ref for an eventing resource.
func EventingTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: base.EventingAPIVersion,
	}
}

// CronJobSourceTypeMeta is the TypeMeta ref for CronJobSource.
var CronJobSourceTypeMeta = SourcesTypeMeta(base.CronJobSourceKind)

// ContainerSourceTypeMeta is the TypeMeta ref for ContainerSource.
var ContainerSourceTypeMeta = SourcesTypeMeta(base.ContainerSourceKind)

// SourcesTypeMeta returns the TypeMeta ref for an eventing sources resource.
func SourcesTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: base.SourcesAPIVersion,
	}
}

// KafkaChannelTypeMeta is the TypeMeta ref for KafkaChannel.
var KafkaChannelTypeMeta = MessagingTypeMeta(base.KafkaChannelKind)

// MessagingTypeMeta returns the TypeMeta ref for an eventing messaing resource.
func MessagingTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: base.MessagingAPIVersion,
	}
}
