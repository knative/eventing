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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
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
	CommonEventTypeSpec `json:",inline"`
	// ClusterEventSource is the name of the ClusterEventSource that produces this ClusterEventType.
	ClusterEventSource string `json:"eventSource"`
}

// ClusterEventTypeStatus is the status for a ClusterEventType resource
type ClusterEventTypeStatus struct {
	CommonEventTypeStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterEventTypeList is a list of ClusterEventType resources
type ClusterEventTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterEventType `json:"items"`
}
