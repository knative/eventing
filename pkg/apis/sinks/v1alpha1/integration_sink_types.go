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
	//	_ apis.Convertible   = (*IntegrationSink)(nil)
)

type IntegrationSinkSpec struct {
	Aws *Aws `json:"aws,omitempty"` // AWS source configuration
}

type AWSCommon struct {
	// Auth is the S3 authentication (accessKey/secretKey) configuration.
	Region string `json:"region,omitempty"` // AWS region
	//UseDefaultCredentials  bool   `json:"useDefaultCredentials" default:"false"` // Use default credentials provider
	//UseProfileCredentials  bool   `json:"useProfileCredentials" default:"false"` // Use profile credentials provider
	ProfileCredentialsName string `json:"profileCredentialsName,omitempty"` // Profile name for profile credentials provider
	//	UseSessionCredentials  bool   `json:"useSessionCredentials" default:"false"` // Use session credentials
	SessionToken        string `json:"sessionToken,omitempty"`           // Session token
	URIEndpointOverride string `json:"uriEndpointOverride,omitempty"`    // Override endpoint URI
	OverrideEndpoint    bool   `json:"overrideEndpoint" default:"false"` // Override endpoint flag
}

type AWSS3 struct {
	AWSCommon
	BucketNameOrArn         string `json:"bucketNameOrArn,omitempty"`         // S3 Bucket name or ARN
	DeleteAfterRead         bool   `json:"deleteAfterRead" default:"true"`    // Auto-delete objects after reading
	MoveAfterRead           bool   `json:"moveAfterRead" default:"false"`     // Move objects after reading
	DestinationBucket       string `json:"destinationBucket,omitempty"`       // Destination bucket for moved objects
	DestinationBucketPrefix string `json:"destinationBucketPrefix,omitempty"` // Prefix for moved objects
	DestinationBucketSuffix string `json:"destinationBucketSuffix,omitempty"` // Suffix for moved objects
	AutoCreateBucket        bool   `json:"autoCreateBucket" default:"false"`  // Auto-create S3 bucket
	Prefix                  string `json:"prefix,omitempty"`                  // S3 bucket prefix for search
	IgnoreBody              bool   `json:"ignoreBody" default:"false"`        // Ignore object body
	ForcePathStyle          bool   `json:"forcePathStyle" default:"false"`    // Force path style for bucket access
	Delay                   int    `json:"delay" default:"500"`               // Delay between polls in milliseconds
	MaxMessagesPerPoll      int    `json:"maxMessagesPerPoll" default:"10"`   // Max messages to poll per request
}

type AWSSQS struct {
	AWSCommon
	QueueNameOrArn     string `json:"queueNameOrArn,omitempty"`              // SQS Queue name or ARN
	DeleteAfterRead    bool   `json:"deleteAfterRead" default:"true"`        // Auto-delete messages after reading
	AutoCreateQueue    bool   `json:"autoCreateQueue" default:"false"`       // Auto-create SQS queue
	AmazonAWSHost      string `json:"amazonAWSHost" default:"amazonaws.com"` // AWS host
	Protocol           string `json:"protocol" default:"https"`              // Communication protocol (http/https)
	QueueURL           string `json:"queueURL,omitempty"`                    // Full SQS queue URL
	Greedy             bool   `json:"greedy" default:"false"`                // Greedy scheduler
	Delay              int    `json:"delay" default:"500"`                   // Delay between polls in milliseconds
	MaxMessagesPerPoll int    `json:"maxMessagesPerPoll" default:"1"`        // Max messages to return (1-10)
	WaitTimeSeconds    int    `json:"waitTimeSeconds,omitempty"`             // Wait time for messages
	VisibilityTimeout  int    `json:"visibilityTimeout,omitempty"`           // Visibility timeout in seconds
}

type Aws struct {
	S3   *AWSS3  `json:"s3,omitempty"`  // S3 source configuration
	SQS  *AWSSQS `json:"sqs,omitempty"` // SQS source configuration
	Auth *Auth   `json:"auth,omitempty"`
}

type Auth struct {
	// Auth Secret
	Secret *Secret `json:"secret,omitempty"`

	// AccessKey is the AWS access key ID.
	AccessKey string `json:"accessKey,omitempty"`

	// SecretKey is the AWS secret access key.
	SecretKey string `json:"secretKey,omitempty"`
}

func (a *Auth) HasAuth() bool {
	return a != nil && a.Secret != nil &&
		a.Secret.Ref != nil && a.Secret.Ref.Name != ""
}

type Secret struct {
	// Secret reference for SASL and SSL configurations.
	Ref *SecretReference `json:"ref,omitempty"`
}

type SecretReference struct {
	// Secret name.
	Name string `json:"name"`
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
