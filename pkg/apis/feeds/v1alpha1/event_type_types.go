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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventType is a specification for a EventType resource
// EventSource can expose multiple event types. For example, github
// has PullRequest events as well as Issues and Comments, etc.
type EventType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EventTypeSpec   `json:"spec"`
	Status EventTypeStatus `json:"status"`
}

// EventTypeSpec specifies information about the EventType, including a schema
// for the event and information about the parameters needed to create a Feed to
// the event.
type EventTypeSpec struct {
	CommonEventTypeSpec `json:",inline"`
	// EventSource is the name of the EventSource that produces this EventType.
	EventSource string `json:"eventSource"`
}

// EventTypeStatus is the status for a EventType resource
type EventTypeStatus struct {
	CommonEventTypeStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventTypeList is a list of EventType resources
type EventTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EventType `json:"items"`
}

func (et *EventType) GetSpecJSON() ([]byte, error) {
	return json.Marshal(et.Spec)
}
