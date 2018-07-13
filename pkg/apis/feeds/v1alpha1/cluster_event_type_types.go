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

// ClusterEventType is a specification for a ClusterEventType resource
// EventSource can expose multiple event types. For example, github
// has PullRequest events as well as Issues and Comments, etc.
type ClusterEventType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterEventTypeSpec   `json:"spec"`
	Status ClusterEventTypeStatus `json:"status"`
}

// ClusterEventTypeSpec specifies information about the ClusterEventType, including a schema
// for the event and information about the parameters needed to create a Feed to
// the event.
type ClusterEventTypeSpec struct {
	// ClusterEventSource is the name of the ClusterEventSource that produces this ClusterEventType.
	ClusterEventSource string `json:"eventSource"`
	// Description is a human-readable description of the ClusterEventType.
	Description string `json:"description,omitempty"`
	// SubscribeSchema describing how to subscribe to this. This basically
	// defines what is required in the Feed.Parameters so that the developer
	// can see the required parameters.
	SubscribeSchema *runtime.RawExtension `json:"subscribeSchema,omitempty"`
	// Describe the schema for the events emitted by this ClusterEventType.
	EventSchema *runtime.RawExtension `json:"eventSchema,omitempty"`
}

// ClusterEventTypeStatus is the status for a ClusterEventType resource
type ClusterEventTypeStatus struct {
	Conditions []ClusterEventTypeCondition `json:"conditions,omitempty"`
}

type ClusterEventTypeConditionType string

const (
	// ClusterEventTypeComplete specifies that the ClusterEventType has completed successfully.
	ClusterEventTypeComplete ClusterEventTypeConditionType = "Complete"
	// ClusterEventTypeFailed specifies that the ClusterEventType has failed.
	ClusterEventTypeFailed ClusterEventTypeConditionType = "Failed"
	// ClusterEventTypeInvalid specifies that the given ClusterEventType specification is invalid.
	ClusterEventTypeInvalid ClusterEventTypeConditionType = "Invalid"
)

// ClusterEventTypeCondition defines a readiness condition for a ClusterEventType.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type ClusterEventTypeCondition struct {
	Type ClusterEventTypeConditionType `json:"state"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterEventTypeList is a list of ClusterEventType resources
type ClusterEventTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterEventType `json:"items"`
}

func (ets *ClusterEventTypeStatus) SetCondition(new *ClusterEventTypeCondition) {
	if new == nil {
		return
	}

	t := new.Type
	var conditions []ClusterEventTypeCondition
	for _, cond := range ets.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	ets.Conditions = conditions
}

func (ets *ClusterEventTypeStatus) RemoveCondition(t ClusterEventTypeConditionType) {
	var conditions []ClusterEventTypeCondition
	for _, cond := range ets.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	ets.Conditions = conditions
}
