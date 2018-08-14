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
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CommonEventTypeSpec specifies information about the [Cluster]EventType,
// including a schema for the event and information about the parameters
// needed to create a Feed to the event.
type CommonEventTypeSpec struct {
	// Description is a human-readable description of the EventType.
	Description string `json:"description,omitempty"`
	// SubscribeSchema describing how to subscribe to this. This basically
	// defines what is required in the Feed.Parameters so that the developer
	// can see the required parameters.
	SubscribeSchema *runtime.RawExtension `json:"subscribeSchema,omitempty"`
	// Describe the schema for the events emitted by this EventType.
	EventSchema *runtime.RawExtension `json:"eventSchema,omitempty"`
}

// Check that CommonEventTypeSpec can be validated and can be defaulted
var _ apis.Validatable = (*CommonEventTypeSpec)(nil)
var _ apis.Defaultable = (*CommonEventTypeSpec)(nil)

// CommonEventTypeStatus is the status for a EventType resource
type CommonEventTypeStatus struct {
	Conditions []CommonEventTypeCondition `json:"conditions,omitempty"`
}

type CommonEventTypeConditionType string

const (
	// EventTypeComplete specifies that the EventType has completed successfully.
	EventTypeComplete CommonEventTypeConditionType = "Complete"
	// EventTypeFailed specifies that the EventType has failed.
	EventTypeFailed CommonEventTypeConditionType = "Failed"
	// EventTypeInvalid specifies that the given EventType specification is invalid.
	EventTypeInvalid CommonEventTypeConditionType = "Invalid"
)

// EventTypeCondition defines a readiness condition for a EventType.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type CommonEventTypeCondition struct {
	Type CommonEventTypeConditionType `json:"type"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

func (ets *CommonEventTypeStatus) SetCondition(new *CommonEventTypeCondition) {
	if new == nil || new.Type == "" {
		return
	}

	t := new.Type
	var conditions []CommonEventTypeCondition
	for _, cond := range ets.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	ets.Conditions = conditions
}

func (ets *CommonEventTypeStatus) RemoveCondition(t CommonEventTypeConditionType) {
	if t == "" {
		return
	}

	var conditions []CommonEventTypeCondition
	for _, cond := range ets.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	ets.Conditions = conditions
}
