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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bind is a specification for a Bind resource
type Bind struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BindSpec   `json:"spec"`
	Status BindStatus `json:"status"`
}

type BindAction struct {
	// RouteName specifies Elafros route as a target.
	RouteName string `json:"routeName,omitempty"`
}

type BindSource struct {
	// EventSource specifies the event source to consume
	EventSource string `json:"eventSource"`

	// EventType specifies the event type to consume
	EventType string `json:"eventType"`
}

// BindSpec is the spec for a Bind resource
type BindSpec struct {
	// Action specifies the target handler for the events
	Action BindAction `json:"action"`

	// Source specifies the trigger we're binding to
	Source BindSource `json:"source"`

	// Parameters is what's necessary to create the subscription.
	// This is specific to each Source. Opaque to platform, only consumed
	// by the actual trigger actuator.
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// ParametersFrom are pointers to secrets that contain sensitive
	// parameters. Opaque to platform, merged in with Parameters and consumed
	// by the actual trigger actuator.
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
	// The name of the secret in the pod's namespace to select from.
	Name string `json:"name"`
	// The key of the secret to select from.  Must be a valid secret key.
	Key string `json:"key"`
}

// BindStatus is the status for a Bind resource
type BindStatus struct {
	Conditions []BindCondition `json:"conditions,omitempty"`
}

type BindConditionType string

const (
	// BindComplete specifies that the bind has completed successfully.
	BindComplete BindConditionType = "Complete"
	// BindFailed specifies that the bind has failed.
	BindFailed BindConditionType = "Failed"
	// BindInvalid specifies that the given bind specification is invalid.
	BindInvalid BindConditionType = "Invalid"
)

// BindCondition defines a readiness condition for a Bind.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type BindCondition struct {
	Type BindConditionType `json:"state"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`
	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BindList is a list of Bind resources
type BindList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Bind `json:"items"`
}

func (bs *BindStatus) SetCondition(new *BindCondition) {
	if new == nil {
		return
	}

	t := new.Type
	var conditions []BindCondition
	for _, cond := range bs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	bs.Conditions = conditions
}

func (bs *BindStatus) RemoveCondition(t BindConditionType) {
	var conditions []BindCondition
	for _, cond := range bs.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	bs.Conditions = conditions
}
