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

type AWSCommon struct {
	// Auth is the S3 authentication (accessKey/secretKey) configuration.
	Region                 string `json:"region,omitempty"`                 // AWS region
	ProfileCredentialsName string `json:"profileCredentialsName,omitempty"` // Profile name for profile credentials provider
	SessionToken           string `json:"sessionToken,omitempty"`           // Session token
	URIEndpointOverride    string `json:"uriEndpointOverride,omitempty"`    // Override endpoint URI
	OverrideEndpoint       bool   `json:"overrideEndpoint" default:"false"` // Override endpoint flag
}

type AWSS3 struct {
	AWSCommon               `json:",inline"` // Embeds AWSCommon to inherit its fields in JSON
	BucketNameOrArn         string           `json:"bucketNameOrArn,omitempty"`         // S3 Bucket name or ARN
	DeleteAfterRead         bool             `json:"deleteAfterRead" default:"true"`    // Auto-delete objects after reading
	MoveAfterRead           bool             `json:"moveAfterRead" default:"false"`     // Move objects after reading
	DestinationBucket       string           `json:"destinationBucket,omitempty"`       // Destination bucket for moved objects
	DestinationBucketPrefix string           `json:"destinationBucketPrefix,omitempty"` // Prefix for moved objects
	DestinationBucketSuffix string           `json:"destinationBucketSuffix,omitempty"` // Suffix for moved objects
	AutoCreateBucket        bool             `json:"autoCreateBucket" default:"false"`  // Auto-create S3 bucket
	Prefix                  string           `json:"prefix,omitempty"`                  // S3 bucket prefix for search
	IgnoreBody              bool             `json:"ignoreBody" default:"false"`        // Ignore object body
	ForcePathStyle          bool             `json:"forcePathStyle" default:"false"`    // Force path style for bucket access
	Delay                   int              `json:"delay" default:"500"`               // Delay between polls in milliseconds
	MaxMessagesPerPoll      int              `json:"maxMessagesPerPoll" default:"10"`   // Max messages to poll per request
}

type AWSSQS struct {
	AWSCommon          `json:",inline"` // Embeds AWSCommon to inherit its fields in JSON
	QueueNameOrArn     string           `json:"queueNameOrArn,omitempty"`              // SQS Queue name or ARN
	DeleteAfterRead    bool             `json:"deleteAfterRead" default:"true"`        // Auto-delete messages after reading
	AutoCreateQueue    bool             `json:"autoCreateQueue" default:"false"`       // Auto-create SQS queue
	AmazonAWSHost      string           `json:"amazonAWSHost" default:"amazonaws.com"` // AWS host
	Protocol           string           `json:"protocol" default:"https"`              // Communication protocol (http/https)
	QueueURL           string           `json:"queueURL,omitempty"`                    // Full SQS queue URL
	Greedy             bool             `json:"greedy" default:"false"`                // Greedy scheduler
	Delay              int              `json:"delay" default:"500"`                   // Delay between polls in milliseconds
	MaxMessagesPerPoll int              `json:"maxMessagesPerPoll" default:"1"`        // Max messages to return (1-10)
	WaitTimeSeconds    int              `json:"waitTimeSeconds,omitempty"`             // Wait time for messages
	VisibilityTimeout  int              `json:"visibilityTimeout,omitempty"`           // Visibility timeout in seconds
}

type AWSDDBStreams struct {
	AWSCommon          `json:",inline"` // Embeds AWSCommon to inherit its fields in JSON
	Table              string           `json:"table,omitempty"`                                    // The name of the DynamoDB table
	StreamIteratorType string           `json:"streamIteratorType,omitempty" default:"FROM_LATEST"` // Defines where in the DynamoDB stream to start getting records
	Delay              int              `json:"delay,omitempty" default:"500"`                      // Delay in milliseconds before the next poll from the database
}

type Aws struct {
	S3         *AWSS3         `json:"s3,omitempty"`          // S3 source configuration
	SQS        *AWSSQS        `json:"sqs,omitempty"`         // SQS source configuration
	DDBStreams *AWSDDBStreams `json:"ddb-streams,omitempty"` // DynamoDB Streams source configuration
	Auth       *Auth          `json:"auth,omitempty"`
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
