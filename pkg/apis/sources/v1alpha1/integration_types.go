/*
Copyright 2020 The Knative Authors

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/common/integration/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IntegrationSource is the Schema for the Integrationsources API
type IntegrationSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IntegrationSourceSpec   `json:"spec,omitempty"`
	Status IntegrationSourceStatus `json:"status,omitempty"`
}

var (
	_ runtime.Object     = (*IntegrationSource)(nil)
	_ kmeta.OwnerRefable = (*IntegrationSource)(nil)
	_ apis.Validatable   = (*IntegrationSource)(nil)
	_ apis.Defaultable   = (*IntegrationSource)(nil)
	_ apis.HasSpec       = (*IntegrationSource)(nil)
	_ duckv1.KRShaped    = (*IntegrationSource)(nil)
	_ apis.Convertible   = (*IntegrationSource)(nil)
)

// IntegrationSourceSpec defines the desired state of IntegrationSource
type IntegrationSourceSpec struct {
	// inherits duck/v1 SourceSpec, which currently provides:
	// * Sink - a reference to an object that will resolve to a domain name or
	//   a URI directly to use as the sink.
	// * CloudEventOverrides - defines overrides to control the output format
	//   and modifications of the event sent to the sink.
	duckv1.SourceSpec `json:",inline"`

	Aws   *Aws   `json:"aws,omitempty"`   // AWS source configuration
	Timer *Timer `json:"timer,omitempty"` // Timer configuration
}

type Timer struct {
	Period      int    `json:"period" default:"1000"`            // Interval (in milliseconds) between producing messages
	Message     string `json:"message"`                          // Message to generate
	ContentType string `json:"contentType" default:"text/plain"` // Content type of generated message
	RepeatCount int    `json:"repeatCount,omitempty"`            // Max number of fires (optional)
}

type Aws struct {
	S3         *v1alpha1.AWSS3         `json:"s3,omitempty"`         // S3 source configuration
	SQS        *v1alpha1.AWSSQS        `json:"sqs,omitempty"`        // SQS source configuration
	DDBStreams *v1alpha1.AWSDDBStreams `json:"ddbStreams,omitempty"` // DynamoDB Streams source configuration
	Auth       *v1alpha1.Auth          `json:"auth,omitempty"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (*IntegrationSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("IntegrationSource")
}

// IntegrationSourceStatus defines the observed state of IntegrationSource
type IntegrationSourceStatus struct {
	// inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IntegrationSourceList contains a list of IntegrationSource
type IntegrationSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IntegrationSource `json:"items"`
}

// GetUntypedSpec returns the spec of the IntegrationSource.
func (c *IntegrationSource) GetUntypedSpec() interface{} {
	return c.Spec
}

// GetStatus retrieves the status of the IntegrationSource. Implements the KRShaped interface.
func (c *IntegrationSource) GetStatus() *duckv1.Status {
	return &c.Status.Status
}
