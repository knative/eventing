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

const (

	// AwsAccessKey is the name of the expected key on the secret for accessing the actual AWS access key value.
	AwsAccessKey = "aws.accessKey"
	// AwsSecretKey is the name of the expected key on the secret for accessing the actual AWS secret key value.
	AwsSecretKey = "aws.secretKey"
)

type AWSCommon struct {
	// Auth is the S3 authentication (accessKey/secretKey) configuration.
	Region              string `json:"region,omitempty"`                 // AWS region
	URIEndpointOverride string `json:"uriEndpointOverride,omitempty"`    // Override endpoint URI
	OverrideEndpoint    bool   `json:"overrideEndpoint" default:"false"` // Override endpoint flag
}

type AWSS3 struct {
	AWSCommon               `json:",inline"` // Embeds AWSCommon to inherit its fields in JSON
	Arn                     string           `json:"arn,omitempty" camel:"CAMEL_KAMELET_AWS_S3_SOURCE_BUCKETNAMEORARN"` // S3 ARN
	DeleteAfterRead         bool             `json:"deleteAfterRead" default:"true"`                                    // Auto-delete objects after reading
	MoveAfterRead           bool             `json:"moveAfterRead" default:"false"`                                     // Move objects after reading
	DestinationBucket       string           `json:"destinationBucket,omitempty"`                                       // Destination bucket for moved objects
	DestinationBucketPrefix string           `json:"destinationBucketPrefix,omitempty"`                                 // Prefix for moved objects
	DestinationBucketSuffix string           `json:"destinationBucketSuffix,omitempty"`                                 // Suffix for moved objects
	AutoCreateBucket        bool             `json:"autoCreateBucket" default:"false"`                                  // Auto-create S3 bucket
	Prefix                  string           `json:"prefix,omitempty"`                                                  // S3 bucket prefix for search
	IgnoreBody              bool             `json:"ignoreBody" default:"false"`                                        // Ignore object body
	ForcePathStyle          bool             `json:"forcePathStyle" default:"false"`                                    // Force path style for bucket access
	Delay                   int              `json:"delay" default:"500"`                                               // Delay between polls in milliseconds
	MaxMessagesPerPoll      int              `json:"maxMessagesPerPoll" default:"10"`                                   // Max messages to poll per request
}

type AWSSQS struct {
	AWSCommon          `json:",inline"` // Embeds AWSCommon to inherit its fields in JSON
	Arn                string           `json:"arn,omitempty" camel:"CAMEL_KAMELET_AWS_SQS_SOURCE_QUEUENAMEORARN"`               // SQS ARN
	DeleteAfterRead    bool             `json:"deleteAfterRead" default:"true"`                                                  // Auto-delete messages after reading
	AutoCreateQueue    bool             `json:"autoCreateQueue" default:"false"`                                                 // Auto-create SQS queue
	Host               string           `json:"host" camel:"CAMEL_KAMELET_AWS_SQS_SOURCE_AMAZONAWSHOST" default:"amazonaws.com"` // AWS host
	Protocol           string           `json:"protocol" default:"https"`                                                        // Communication protocol (http/https)
	QueueURL           string           `json:"queueURL,omitempty"`                                                              // Full SQS queue URL
	Greedy             bool             `json:"greedy" default:"false"`                                                          // Greedy scheduler
	Delay              int              `json:"delay" default:"500"`                                                             // Delay between polls in milliseconds
	MaxMessagesPerPoll int              `json:"maxMessagesPerPoll" default:"1"`                                                  // Max messages to return (1-10)
	WaitTimeSeconds    int              `json:"waitTimeSeconds,omitempty"`                                                       // Wait time for messages
	VisibilityTimeout  int              `json:"visibilityTimeout,omitempty"`                                                     // Visibility timeout in seconds
}

type AWSDDBStreams struct {
	AWSCommon          `json:",inline"` // Embeds AWSCommon to inherit its fields in JSON
	Table              string           `json:"table,omitempty"`                                    // The name of the DynamoDB table
	StreamIteratorType string           `json:"streamIteratorType,omitempty" default:"FROM_LATEST"` // Defines where in the DynamoDB stream to start getting records
	Delay              int              `json:"delay,omitempty" default:"500"`                      // Delay in milliseconds before the next poll from the database
}

type AWSSNS struct {
	AWSCommon       `json:",inline"` // Embeds AWSCommon to inherit its fields in JSON
	Arn             string           `json:"arn,omitempty" camel:"CAMEL_KAMELET_AWS_SNS_SINK_TOPICNAMEORARN"` // SNS ARN
	AutoCreateTopic bool             `json:"autoCreateTopic" default:"false"`                                 // Auto-create SNS topic
}
