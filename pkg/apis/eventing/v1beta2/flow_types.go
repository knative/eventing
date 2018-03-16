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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Flow is a specification for a Flow resource
type Flow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlowSpec   `json:"spec"`
	Status FlowStatus `json:"status"`
}

type FilterLanguage string

const (
	FilterLanguageUnknown          FilterLanguage = "unknown"
	FilterPathMatch                FilterLanguage = "path_match"
	FilterCommonExpressionLanguage FilterLanguage = "common_expression_language"
)

type Filter struct {
	Language   FilterLanguage `json:"language"`
	Expression string         `json:"expression"`
}

type Source struct {
	// SourceID is source-id from CNCF event model
	SourceID string `json:"sourceId"`
	Provider string `json:"provider"`
}

type Action struct {
	Name      string `json:"name"`
	Processor string `json:"processor"`
	EventType string `json"eventType"`
}

// FlowSpec is the spec for a Flow resource
type FlowSpec struct {
	EventType string `json:"eventType"`

	// Source specifies the event source to consume
	Source Source `json:"source,omitempty"`

	// Action to call
	Action Action `json:"action"`

	// Filters for emitting an event, all of which must evaluate to true
	Filters []Filter `json:"flowConditions"`
}

// FlowStatus is the status for a Flow resource
type FlowStatus struct {
	Conditions []FlowCondition `json:"conditions,omitempty"`
}

type FlowConditionType string

const (
	// FlowComplete specifies that the bind has completed successfully.
	FlowComplete FlowConditionType = "Complete"
	// FlowFailed specifies that the bind has failed.
	FlowFailed FlowConditionType = "Failed"
	// FlowInvalid specifies that the given bind specification is invalid.
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
