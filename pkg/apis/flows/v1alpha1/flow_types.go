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

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Flow connects an event source with an action that processes events produced
// by the event source.
type Flow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlowSpec   `json:"spec"`
	Status FlowStatus `json:"status"`
}

// FlowSpec is the spec for a Flow resource.
type FlowSpec struct {
	// Action specifies the target handler for the events
	Action FlowAction `json:"action"`

	// Trigger specifies the trigger we're creating a Flow to
	Trigger EventTrigger `json:"trigger"`

	// Service Account to use when creating the underlying Feed.
	// If left out, uses "default"
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// FlowAction specifies the reference to an object that's expected to
// provide the resolved target of the action. Currently we inspect
// the objects Status and see if there's a predefined Status field
// that we will then use to give to Feed object as the target. Currently
// must resolve to a k8s service or Istio virtual service. Note that by
// in the future we should try to utilize subresources (/resolve ?) to
// utilize this, but CRDs do not support subresources yet, so we need
// to rely on a specified Status field today. By relying on this behaviour
// we can utilize a dynamic client instead of having to understand all
// kinds of different types of objects. As long as they adhere to this
// particular contract, they can be used as a Target.
// To ensure that we can support external targets and for ease of use
// we also allow for an URI to be specified.
type FlowAction struct {
	// Only one of these can be specified

	// Reference to an object that will be used to find the target
	// endpoint.
	// For example, this could be a reference to a Route resource
	// or a Configuration resource.
	// TODO: Specify the required fields the target object must
	// have in the status.
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// +optional
	Target *corev1.ObjectReference `json:"target,omitempty"`

	// Reference to a 'known' endpoint where no resolving be done.
	// http://k8s-service for example
	// http://myexternalhandler.example.com/foo/bar
	// +optional
	TargetURI *string `json:"targetURI,omitempty"`
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
	ParametersFrom []feedsv1alpha1.ParametersFromSource `json:"parametersFrom,omitempty"`
}

// FlowStatus is the status for a Flow resource
type FlowStatus struct {
	Conditions []FlowCondition `json:"conditions,omitempty"`

	// FlowContext is what the Flow operation returns and holds enough information
	// to perform cleanup once a Flow is deleted.
	// NOTE: experimental field.
	FlowContext *runtime.RawExtension `json:"flowContext,omitempty"`

	// ChannelTarget is the name of the target channel
	ChannelTarget string `json:"channelTarget,omitempty"`

	// ObservedGeneration is the 'Generation' of the Flow that
	// was last reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type FlowConditionType string

const (
	// FlowConditionReady specifies that the Flow has been configured successfully.
	FlowConditionReady FlowConditionType = "Ready"

	// FlowConditionFeedReady specifies that the Feed has been configured successfully.
	FlowConditionFeedReady FlowConditionType = "FeedReady"

	// FlowConditionChannelReady specifies that the Channel has been configured successfully.
	FlowConditionChannelReady FlowConditionType = "ChannelReady"

	// FlowConditionSubscriptionReady specifies that the Subscription has been configured successfully.
	FlowConditionSubscriptionReady FlowConditionType = "SubscriptionReady"
)

// FlowCondition defines a readiness condition for a Flow.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type FlowCondition struct {
	Type FlowConditionType `json:"type"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`
	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FlowList is a list of Flow resources
type FlowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Flow `json:"items"`
}

func (fs *FlowStatus) IsReady() bool {
	if c := fs.GetCondition(FlowConditionReady); c != nil {
		return c.Status == corev1.ConditionTrue
	}
	return false
}

func (fs *FlowStatus) GetCondition(t FlowConditionType) *FlowCondition {
	for _, cond := range fs.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

func (fs *FlowStatus) setCondition(new *FlowCondition) {
	if new == nil || new.Type == "" {
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

func (fs *FlowStatus) removeCondition(t FlowConditionType) {
	var conditions []FlowCondition
	for _, cond := range fs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	fs.Conditions = conditions
}

func (fs *FlowStatus) PropagateChannelStatus(cs channelsv1alpha1.ChannelStatus) {
	if cs.DomainInternal != "" {
		fs.ChannelTarget = cs.DomainInternal
		fs.setCondition(&FlowCondition{
			Type:   FlowConditionChannelReady,
			Status: corev1.ConditionTrue,
		})
		fs.checkAndMarkReady()
	}
}

func (fs *FlowStatus) PropagateSubscriptionStatus(ss channelsv1alpha1.SubscriptionStatus) {
	// TODO: Once SubscriptionStatus has meaningful content, add it here.
}

func (fs *FlowStatus) PropagateFeedStatus(s feedsv1alpha1.FeedStatus) {
	// Check to see if Feed is ready
	fc := s.GetCondition(feedsv1alpha1.FeedConditionReady)

	if fc == nil {
		return
	}
	fst := []FlowConditionType{FlowConditionFeedReady}
	// If the underlying Feed reported not ready, then bubble it up.
	if fc.Status != corev1.ConditionTrue {
		fst = append(fst, FlowConditionReady)
	}
	for _, cond := range fst {
		fs.setCondition(&FlowCondition{
			Type:    cond,
			Status:  fc.Status,
			Reason:  fc.Reason,
			Message: fc.Message,
		})
	}
	fs.checkAndMarkReady()
}

func (fs *FlowStatus) checkAndMarkReady() {
	for _, cond := range []FlowConditionType{
		FlowConditionFeedReady,
		FlowConditionChannelReady,
	} {
		c := fs.GetCondition(cond)
		if c == nil || c.Status != corev1.ConditionTrue {
			return
		}
	}
	fs.markReady()
}

func (fs *FlowStatus) markReady() {
	fs.setCondition(&FlowCondition{
		Type:   FlowConditionReady,
		Status: corev1.ConditionTrue,
	})
}
