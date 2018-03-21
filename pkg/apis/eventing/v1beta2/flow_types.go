/*
Copyright 2018 Google, Inc. All rights reserved.

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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Flow is a binding of a Source to an Action specifying the trigger
// condition and the event type.
type Flow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlowSpec   `json:"spec"`
	Status FlowStatus `json:"status"`
}

// Action is a consumer of events running in a Processor, for example a
// particular Google Cloud Function.
type Action struct {
	// Name is where the event will be delivered to, for example
	// "projects/my-project-id/locations/mordor-central1/functions/functionName"
	Name string `json:"name"`

	// Where the action runs. For example "google.cloud.functons" or "http".
	Processor string `json:"processor"`
}

// EventTrigger represents an interest in a subsets of events occurring in
// a service.
type EventTrigger struct {
	// EventType is the type of event to observe. For example:
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
	//     (-- GOOGLE_INTERNAL Within Google, subdivisions match the directory
	//     structure under `google3/google/...`, ending before any
	//     version number --)
	//  2. resource type: The type of resource on which event occurs. For
	//     example, the Google Cloud Storage API includes the types `object`
	//     and `bucket`.
	//  3. action: The action that generates the event. For example, actions for
	//     a Google Cloud Storage Object include 'finalize' and 'delete'.
	// These parts are lower case and joined by '.'.
	EventType string `json:"event_type"`

	// Resource or Resources from which to observe events, for example,
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

	// Service is the hostname of the service that should be observed.
	//
	// If no string is provided, the default service implementing the API will
	// be used. For example, `storage.googleapis.com` is the default for all
	// event types in the 'google.storage` namespace.
	// (-- GOOGLE_INTERNAL:
	//  Use this string to target staging versions of your service.
	// --)
	Service string `json:"service"`
}

// FlowSpec is the spec for a Flow resource
type FlowSpec struct {
	// Trigger contains the event_type, the "resource" path, and the hostname of the
	// service hosting the event source. The "resource" includes the event source
	// and a path match expression specifing a condition for emitting an event.
	Trigger EventTrigger `json:"trigger"`

	// Action is where an event gets delivered to. For example an HTTP endpoint.
	Action Action `json:"action"`
}

// FlowStatus is the status for a Flow resource
type FlowStatus struct {
	Conditions []FlowCondition `json:"conditions,omitempty"`
}

type FlowConditionType string

const (
	// FlowComplete specifies that the flow has completed successfully.
	FlowComplete FlowConditionType = "Complete"
	// FlowFailed specifies that the flow has failed.
	FlowFailed FlowConditionType = "Failed"
	// FlowInvalid specifies that the given flow specification is invalid.
	FlowInvalid FlowConditionType = "Invalid"
)

// FlowCondition defines a readiness condition for a Flow.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type FlowCondition struct {
	Type FlowConditionType `json:"state"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" description:"last time the condition transit from one status to another"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// FlowList is a list of Flow resources
type FlowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Flow `json:"items"`
}

func (fs *FlowStatus) SetCondition(new *FlowCondition) {
	if new == nil {
		return
	}

	t := new.Type
	var conditions []FlowCondition
	for _, cond := range fs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	fs.Conditions = conditions
}

func (fs *FlowStatus) RemoveCondition(t FlowConditionType) {
	var conditions []FlowCondition
	for _, cond := range fs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	fs.Conditions = conditions
}
