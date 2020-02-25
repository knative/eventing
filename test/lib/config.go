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

// DefaultChannel is the default channel we will run tests against.
var DefaultChannel = InMemoryChannelTypeMeta

// InMemoryChannelTypeMeta is the metav1.TypeMeta for InMemoryChannel.
var InMemoryChannelTypeMeta = metav1.TypeMeta{
	APIVersion: resources.MessagingAPIVersion,
	Kind:       resources.InMemoryChannelKind,
}

// MessagingChannelTypeMeta is the metav1.TypeMeta for messaging.Channel.
var MessagingChannelTypeMeta = metav1.TypeMeta{
	APIVersion: resources.MessagingAPIVersion,
	Kind:       resources.ChannelKind,
}

// ChannelFeatureMap saves the channel-features mapping.
// Each pair means the channel support the list of features.
var ChannelFeatureMap = map[metav1.TypeMeta][]Feature{
	InMemoryChannelTypeMeta:  {FeatureBasic},
	MessagingChannelTypeMeta: {FeatureBasic},
}

// Feature is the feature supported by the channel.
type Feature string

const (
	// FeatureBasic is the feature that should be supported by all channels.
	FeatureBasic Feature = "basic"
	// FeatureRedelivery means if downstream rejects an event, that request will be attempted again.
	FeatureRedelivery Feature = "redelivery"
	// FeaturePersistence means if channel's Pod goes down, all events already ACKed by the channel
	// will persist and be retransmitted when the Pod restarts.
	FeaturePersistence Feature = "persistence"
)
