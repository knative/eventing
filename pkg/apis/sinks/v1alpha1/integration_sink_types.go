/*
Copyright 2024 The Knative Authors

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
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// IntegrationSink is the Schema for the IntegrationSink API.
type IntegrationSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IntegrationSinkSpec   `json:"spec,omitempty"`
	Status IntegrationSinkStatus `json:"status,omitempty"`
}

// Check the interfaces that JobSink should be implementing.
var (
	_ runtime.Object     = (*IntegrationSink)(nil)
	_ kmeta.OwnerRefable = (*IntegrationSink)(nil)
	_ apis.Validatable   = (*IntegrationSink)(nil)
	_ apis.Defaultable   = (*IntegrationSink)(nil)
	_ apis.HasSpec       = (*IntegrationSink)(nil)
	_ duckv1.KRShaped    = (*IntegrationSink)(nil)
	_ apis.Convertible   = (*JobSink)(nil)
)

type IntegrationSinkSpec struct {
	Aws *Aws `json:"aws,omitempty"` // AWS source configuration
	Log *Log `json:"log,omitempty"` // Log sink configuration
}

type Log struct {
	LoggerName          string `json:"loggerName,omitempty" default:"log-sink"`      // Name of the logging category to use
	Level               string `json:"level,omitempty" default:"INFO"`               // Logging level to use
	LogMask             bool   `json:"logMask,omitempty" default:"false"`            // Mask sensitive information in the log
	Marker              string `json:"marker,omitempty"`                             // An optional Marker name to use
	Multiline           bool   `json:"multiline,omitempty" default:"false"`          // If enabled, outputs each information on a newline
	ShowAllProperties   bool   `json:"showAllProperties,omitempty" default:"false"`  // Show all of the exchange properties (both internal and custom)
	ShowBody            bool   `json:"showBody,omitempty" default:"true"`            // Show the message body
	ShowBodyType        bool   `json:"showBodyType,omitempty" default:"true"`        // Show the body Java type
	ShowExchangePattern bool   `json:"showExchangePattern,omitempty" default:"true"` // Show the Message Exchange Pattern (MEP)
	ShowHeaders         bool   `json:"showHeaders,omitempty" default:"false"`        // Show the headers received
	ShowProperties      bool   `json:"showProperties,omitempty" default:"false"`     // Show the exchange properties (only custom)
	ShowStreams         bool   `json:"showStreams,omitempty" default:"false"`        // Show the stream bodies
	ShowCachedStreams   bool   `json:"showCachedStreams,omitempty" default:"true"`   // Show cached stream bodies
}

type Aws struct {
	S3   *v1alpha1.AWSS3  `json:"s3,omitempty"`  // S3 source configuration
	SQS  *v1alpha1.AWSSQS `json:"sqs,omitempty"` // SQS source configuration
	SNS  *v1alpha1.AWSSNS `json:"sns,omitempty"` // SNS source configuration
	Auth *v1alpha1.Auth   `json:"auth,omitempty"`
}

type IntegrationSinkStatus struct {
	duckv1.Status `json:",inline"`

	// AddressStatus is the part where the JobSink fulfills the Addressable contract.
	// It exposes the endpoint as an URI to get events delivered.
	// +optional
	duckv1.AddressStatus `json:",inline"`

	// AppliedEventPoliciesStatus contains the list of EventPolicies which apply to this JobSink
	// +optional
	eventingduckv1.AppliedEventPoliciesStatus `json:",inline"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (*IntegrationSink) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("IntegrationSink")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IntegrationSinkList contains a list of IntegrationSink
type IntegrationSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IntegrationSink `json:"items"`
}

// GetUntypedSpec returns the spec of the IntegrationSink.
func (c *IntegrationSink) GetUntypedSpec() interface{} {
	return c.Spec
}

// GetStatus retrieves the status of the IntegrationSink. Implements the KRShaped interface.
func (c *IntegrationSink) GetStatus() *duckv1.Status {
	return &c.Status.Status
}
