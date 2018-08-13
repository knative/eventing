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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// CommonEventSourceSpec describes the type and source of an event, a container image
// to run for feed lifecycle operations, and configuration options common for
// ClusterEventSource and EventSource.
type CommonEventSourceSpec struct {
	// Source is the name of the source that produces the events.
	Source string `json:"source,omitempty"`

	// Image is the container image to run for feed lifecycle operations.
	//
	// TODO: make this a container
	// TODO: specify exactly when containers are run
	Image string `json:"image,omitempty"`

	// Parameters are configuration options for a particular EventSource
	// TODO: Consider instead using ConfigMaps and mount them instead
	// on the event sources containers.
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`
}

// Check that CommonEventSourceSpec can be validated and can be defaulted
var _ apis.Validatable = (*CommonEventSourceSpec)(nil)
var _ apis.Defaultable = (*CommonEventSourceSpec)(nil)

// EventSourceStatus is the status for a EventSource resource
type CommonEventSourceStatus struct {
	Conditions []CommonEventSourceCondition `json:"conditions,omitempty"`
}

type CommonEventSourceConditionType string

const (
	// EventSourceComplete specifies that the EventSource has completed successfully.
	EventSourceComplete CommonEventSourceConditionType = "Complete"
	// EventSourceFailed specifies that the EventSource has failed.
	EventSourceFailed CommonEventSourceConditionType = "Failed"
	// EventSourceInvalid specifies that the given EventSource specification is invalid.
	EventSourceInvalid CommonEventSourceConditionType = "Invalid"
)

// CommonEventSourceCondition defines a readiness condition for a EventSource.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type CommonEventSourceCondition struct {
	Type CommonEventSourceConditionType `json:"type"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

func (ess *CommonEventSourceStatus) SetCondition(new *CommonEventSourceCondition) {
	if new == nil || new.Type == "" {
		return
	}

	t := new.Type
	var conditions []CommonEventSourceCondition
	for _, cond := range ess.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	ess.Conditions = conditions
}

func (ess *CommonEventSourceStatus) RemoveCondition(t CommonEventSourceConditionType) {
	if t == "" {
		return
	}

	var conditions []CommonEventSourceCondition
	for _, cond := range ess.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	ess.Conditions = conditions
}
