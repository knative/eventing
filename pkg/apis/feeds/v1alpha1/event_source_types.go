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
	"encoding/json"
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventSource represents a software system which wishes to make changes in
// state discoverable via eventing, without prior knowledge of systems which
// might consume state changes. EventSources produce events that the Feed
// resource connects to consumers.
type EventSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EventSourceSpec   `json:"spec"`
	Status EventSourceStatus `json:"status"`
}

// Check that EventSource can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*EventSource)(nil)
var _ apis.Defaultable = (*EventSource)(nil)
var _ apis.Immutable = (*EventSource)(nil)
var _ runtime.Object = (*EventSource)(nil)
var _ webhook.GenericCRD = (*EventSource)(nil)

// EventSourceSpec describes the type and source of an event, a container image
// to run for feed lifecycle operations, and configuration options for the
// EventSource.
type EventSourceSpec struct {
	CommonEventSourceSpec `json:",inline"`
}

// EventSourceStatus is the status for a EventSource resource
type EventSourceStatus struct {
	CommonEventSourceStatus `json:",inline"`
}

func (es *EventSource) GetSpecJSON() ([]byte, error) {
	return json.Marshal(es.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventSourceList is a list of EventSource resources
type EventSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EventSource `json:"items"`
}
