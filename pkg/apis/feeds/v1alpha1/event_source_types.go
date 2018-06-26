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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventSource is a specification for a EventSource resource
// It would define necessary information to talk to the source of the event.
// Some examples would be github.com, or GCP, etc.
type EventSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EventSourceSpec   `json:"spec"`
	Status EventSourceStatus `json:"status"`
}

// EventSourceSpec is the spec for a EventSource resource
// TODO: Define this to actually be useful. Placeholder for now
type EventSourceSpec struct {
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`

	// Image that we run for bind/unbind operations
	Image string `json:"image,omitempty"`

	// Parameters are configuration options for a particular EventSource
	// TODO: Consider instead using ConfigMaps and mount them instead
	// on the event sources containers.
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`
}

// EventSourceStatus is the status for a EventSource resource
type EventSourceStatus struct {
	Conditions []EventSourceCondition `json:"conditions,omitempty"`
}

type EventSourceConditionType string

const (
	// EventSourceComplete specifies that the bind has completed successfully.
	EventSourceComplete EventSourceConditionType = "Complete"
	// EventSourceFailed specifies that the bind has failed.
	EventSourceFailed EventSourceConditionType = "Failed"
	// EventSourceInvalid specifies that the given bind specification is invalid.
	EventSourceInvalid EventSourceConditionType = "Invalid"
)

// EventSourceCondition defines a readiness condition for a EventSource.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type EventSourceCondition struct {
	Type EventSourceConditionType `json:"state"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventSourceList is a list of EventSource resources
type EventSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EventSource `json:"items"`
}

func (ess *EventSourceStatus) SetCondition(new *EventSourceCondition) {
	if new == nil {
		return
	}

	t := new.Type
	var conditions []EventSourceCondition
	for _, cond := range ess.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	ess.Conditions = conditions
}

func (ess *EventSourceStatus) RemoveCondition(t EventSourceConditionType) {
	var conditions []EventSourceCondition
	for _, cond := range ess.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	ess.Conditions = conditions
}
