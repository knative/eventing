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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Feed connects an event trigger with an action that processes events produced
// by the trigger.
type Feed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeedSpec   `json:"spec"`
	Status FeedStatus `json:"status"`
}

// FeedSpec is the spec for a Feed resource.
type FeedSpec struct {
	// Action specifies the target handler for the events
	Action FeedAction `json:"action"`

	// Trigger specifies the trigger producing events for this feed
	Trigger EventTrigger `json:"trigger"`

	// Service Account to run feed container. If left out, uses "default"
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// FeedAction specifies the target handler - a Knative route or channel - for
// events produced by an EventTrigger.
type FeedAction struct {
	// You must specify one and only of these.

	// RouteName specifies Knative route as a target.
	RouteName string `json:"routeName,omitempty"`

	// ChannelName specifies the channel name as a target
	// If ChannelName specifies a full DNS (for example:
	// flow-example-channel.default.svc.cluster.local)
	// it's returned as is.
	// TODO: clean up the action names.
	ChannelName string `json:"channelName,omitempty"`
}

// EventTrigger specifies the intention that a particular event type and
// resource should be consumed.
type EventTrigger struct {
	// Required. The type of event to observe. For example:
	// `google.storage.object.finalize` and
	// `google.firebase.analytics.event.log`.
	//
	// Event type consists of three parts:
	//  1. namespace: The domain name of the organization in reverse-domain
	//     notation (e.g. `acme.net` appears as `net.acme`) and any orginization
	//     specific subdivisions. If the organization's top-level domain is `com`,
	//     the top-level domain is ommited (e.g. `google.com` appears as
	//     `google`). For example, `google.storage` and
	//     `google.firebase.analytics`.
	//  2. resource type: The type of resource on which event occurs. For
	//     example, the Google Cloud Storage API includes the types `object`
	//     and `bucket`.
	//  3. action: The action that generates the event. For example, actions for
	//     a Google Cloud Storage Object include 'finalize' and 'delete'.
	// These parts are lower case and joined by '.'.
	EventType string `json:"eventType"`

	// Required. The resource(s) from which to observe events, for example,
	// `projects/_/buckets/myBucket/objects/{objectPath=**}`.
	//
	// Can be a specific resource or use wildcards to match a set of resources.
	// Wildcards can either match a single segment in the resource name,
	// using '*', or multiple segments, using '**'. For example,
	// `projects/myProject/buckets/*/objects/**` would match all objects in all
	// buckets in the 'myProject' project.
	//
	// The contents of wildcards can also be captured. This is done by assigning
	// it to a variable name in braces. For example,
	// `projects/myProject/buckets/{bucket_id=*}/objects/{object_path=**}`.
	// Additionally, a single segment capture can omit `=*` and a multiple segment
	// capture can specify additional structure. For example, the following
	// all match the same buckets, but capture different data:
	//     `projects/myProject/buckets/*/objects/users/*/data/**`
	//     `projects/myProject/buckets/{bucket_id=*}/objects/users/{user_id}/data/{data_path=**}`
	//     `projects/myProject/buckets/{bucket_id}/objects/{object_path=users/*/data/**}`
	//
	// Not all syntactically correct values are accepted by all services. For
	// example:
	//
	// 1. The authorization model must support it. Google Cloud Functions
	//    only allows EventTriggers to be deployed that observe resources in the
	//    same project as the `CloudFunction`.
	// 2. The resource type must match the pattern expected for an
	//    `event_type`. For example, an `EventTrigger` that has an
	//    `event_type` of "google.pubsub.topic.publish" should have a resource
	//    that matches Google Cloud Pub/Sub topics.
	//
	// Additionally, some services may support short names when creating an
	// `EventTrigger`. These will always be returned in the normalized "long"
	// format.
	//
	// See each *service's* documentation for supported formats.
	Resource string `json:"resource"`

	// The hostname of the service that should be observed.
	//
	// If no string is provided, the default service implementing the API will
	// be used. For example, `storage.googleapis.com` is the default for all
	// event types in the 'google.storage` namespace.
	Service string `json:"service"`

	// Parameters is what's necessary to create the subscription.
	// This is specific to each Source. Opaque to platform, only consumed
	// by the actual trigger actuator.
	// NOTE: experimental field.
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// ParametersFrom are pointers to secrets that contain sensitive
	// parameters. Opaque to platform, merged in with Parameters and consumed
	// by the actual trigger actuator.
	// NOTE: experimental field. All secrets in ParametersFrom will be
	// resolved and given to event sources in the Parameters field.
	ParametersFrom []ParametersFromSource `json:"parametersFrom,omitempty"`
}

// ParametersFromSource represents the source of a set of Parameters
// TODO: consider making this into a new secret type.
type ParametersFromSource struct {
	// The Secret key to select from.
	// The value must be a JSON object.
	//+optional
	SecretKeyRef *SecretKeyReference `json:"secretKeyRef,omitempty"`
}

// SecretKeyReference references a key of a Secret.
type SecretKeyReference struct {
	// The name of the secret in the resource's namespace to select from.
	Name string `json:"name"`
	// The key of the secret to select from.  Must be a valid secret key.
	Key string `json:"key"`
}

// FeedStatus is the status for a Feed resource
type FeedStatus struct {
	Conditions []FeedCondition `json:"conditions,omitempty"`

	// FeedContext is what the Feed operation returns and holds enough information
	// for the event source to stop the Feed.
	// This is specific to each Feed. Opaque to platform, only consumed
	// by the actual trigger actuator.
	// NOTE: experimental field.
	FeedContext *runtime.RawExtension `json:"feedContext,omitempty"`
}

type FeedConditionType string

const (
	// FeedConditionReady specifies that the feed has started successfully.
	FeedConditionReady FeedConditionType = "Ready"
)

// FeedCondition defines a readiness condition for a Feed.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type FeedCondition struct {
	Type FeedConditionType `json:"type"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`
	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FeedList is a list of Feed resources
type FeedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Feed `json:"items"`
}

func (fs *FeedStatus) GetCondition(t FeedConditionType) *FeedCondition {
	for _, cond := range fs.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

func (fs *FeedStatus) SetCondition(new *FeedCondition) {
	if new == nil || new.Type == "" {
		return
	}

	t := new.Type
	var conditions []FeedCondition
	for _, cond := range fs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	fs.Conditions = conditions
}

func (fs *FeedStatus) RemoveCondition(t FeedConditionType) {
	var conditions []FeedCondition
	for _, cond := range fs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	fs.Conditions = conditions
}
