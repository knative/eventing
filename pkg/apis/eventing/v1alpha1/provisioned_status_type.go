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

// ProvisionedStatus describes the provisioning status of a resource at a point in time.
type ProvisionedStatus struct {
	// Name of resource.
	// +required
	Name string `json:"name"`

	// Type is the Fully Qualified Object type.
	// +required
	Type string `json:"type"`

	// Status of the provisioning
	// +required
	Status string `json:"status"`

	// The last time this condition was updated.
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is the detailed description describing current relationship status.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about the status.
	// +optional
	Message string `json:"message,omitempty"`
}
