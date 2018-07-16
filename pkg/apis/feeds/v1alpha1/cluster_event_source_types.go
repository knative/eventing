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
	CommonEventSourceSpec `json:",inline"`
}

// ClusterEventSourceStatus is the status for a ClusterEventSource resource
type ClusterEventSourceStatus struct {
	CommonEventSourceStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterEventSourceList is a list of ClusterEventSource resources
type ClusterEventSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterEventSource `json:"items"`
}
