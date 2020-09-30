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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// API versions for the resources.
const (
	CoreAPIVersion        = "v1"
	EventingAPIVersion    = "eventing.knative.dev/v1beta1"
	MessagingAPIVersion   = "messaging.knative.dev/v1beta1"
	FlowsAPIVersion       = "flows.knative.dev/v1beta1"
	ServingAPIVersion     = "serving.knative.dev/v1"
	SourcesV1A2APIVersion = "sources.knative.dev/v1alpha2"
	SourcesV1B1APIVersion = "sources.knative.dev/v1beta1"
	SourcesV1APIVersion   = "sources.knative.dev/v1"
)

// Kind for Knative resources.
const (
	KServiceKind string = "Service"
)

var (
	// KServicesGVR is GroupVersionResource for Knative Service
	KServicesGVR = schema.GroupVersionResource{
		Group:    "serving.knative.dev",
		Version:  "v1",
		Resource: "services",
	}
	// KServiceType is type of Knative Service
	KServiceType = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: KServicesGVR.GroupVersion().String(),
	}
)

// Kind for core Kubernetes resources.
const (
	ServiceKind string = "Service"
)

// Kind for eventing resources.
const (
	SubscriptionKind string = "Subscription"

	BrokerKind  string = "Broker"
	TriggerKind string = "Trigger"
)

// Kind for messaging resources.
const (
	InMemoryChannelKind string = "InMemoryChannel"

	ChannelKind  string = "Channel"
	SequenceKind string = "Sequence"
	ParallelKind string = "Parallel"
)

// Kind for flows resources.
const (
	FlowsSequenceKind string = "Sequence"
	FlowsParallelKind string = "Parallel"
)

//Kind for sources resources that exist in Eventing core
const (
	ApiServerSourceKind string = "ApiServerSource"
	PingSourceKind      string = "PingSource"
)
