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
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterEventSource represents a software system which wishes to make changes in
// state discoverable via eventing, without prior knowledge of systems which
// might consume state changes. ClusterEventSources produce events that the Feed
// resource connects to consumers.
type ClusterEventSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterEventSourceSpec   `json:"spec"`
	Status ClusterEventSourceStatus `json:"status"`
}

// ClusterEventSourceSpec describes the type and source of an event, a container image
// to run for feed lifecycle operations, and configuration options for the
// ClusterEventSource.
type ClusterEventSourceSpec struct {
	// Source is the name of the source that produces the events.
	Source string `json:"source,omitempty"`

	// Image is the container image to run for feed lifecycle operations.
	//
	// TODO: make this a container
	// TODO: specify exactly when containers are run
	Image string `json:"image,omitempty"`

	// Parameters are configuration options for a particular ClusterEventSource
	// TODO: Consider instead using ConfigMaps and mount them instead
	// on the event sources containers.
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`
}

// ClusterEventSourceStatus is the status for a ClusterEventSource resource
type ClusterEventSourceStatus struct {
	Conditions []ClusterEventSourceCondition `json:"conditions,omitempty"`
}

type ClusterEventSourceConditionType string

const (
	// ClusterEventSourceComplete specifies that the ClusterEventSource has completed successfully.
	ClusterEventSourceComplete ClusterEventSourceConditionType = "Complete"
	// ClusterEventSourceFailed specifies that the ClusterEventSource has failed.
	ClusterEventSourceFailed ClusterEventSourceConditionType = "Failed"
	// ClusterEventSourceInvalid specifies that the given ClusterEventSource specification is invalid.
	ClusterEventSourceInvalid ClusterEventSourceConditionType = "Invalid"
)

// ClusterEventSourceCondition defines a readiness condition for a ClusterEventSource.
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
type ClusterEventSourceCondition struct {
	Type ClusterEventSourceConditionType `json:"state"`

	Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterEventSourceList is a list of ClusterEventSource resources
type ClusterEventSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterEventSource `json:"items"`
}

func (ess *ClusterEventSourceStatus) SetCondition(new *ClusterEventSourceCondition) {
	if new == nil {
		return
	}

	t := new.Type
	var conditions []ClusterEventSourceCondition
	for _, cond := range ess.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	conditions = append(conditions, *new)
	ess.Conditions = conditions
}

func (ess *ClusterEventSourceStatus) RemoveCondition(t ClusterEventSourceConditionType) {
	var conditions []ClusterEventSourceCondition
	for _, cond := range ess.Conditions {
		if cond.Type != t {
			conditions = append(conditions, cond)
		}
	}
	ess.Conditions = conditions
}
