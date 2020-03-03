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

// SystemNamespace is the namespace where Eventing is installed, it's default to be knative-eventing.
const SystemNamespace = "knative-eventing"

// API versions for the resources.
const (
	CoreAPIVersion      = "v1"
	EventingAPIVersion  = "eventing.knative.dev/v1alpha1"
	MessagingAPIVersion = "messaging.knative.dev/v1alpha1"
	FlowsAPIVersion     = "flows.knative.dev/v1alpha1"
	ServingAPIVersion   = "serving.knative.dev/v1"
)

// Kind for Knative resources.
const (
	KServiceKind string = "Service"
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

// Kind for sources resources.
const (
	CronJobSourceKind   string = "CronJobSource"
	ContainerSourceKind string = "ContainerSource"
	ApiServerSourceKind string = "ApiServerSource"
)
