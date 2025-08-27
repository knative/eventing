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
	"context"
	"testing"

	"knative.dev/eventing/pkg/apis/common/integration/v1alpha1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

func TestIntegrationSinkSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		spec IntegrationSinkSpec
		want *apis.FieldError
	}{
		{
			name: "valid log sink",
			spec: IntegrationSinkSpec{
				Log: &Log{
					Level:       "info",
					ShowHeaders: true,
				},
			},
			want: nil,
		},
		{
			name: "valid AWS S3 sink with auth and region",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &v1alpha1.AWSS3{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-bucket",
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "valid AWS S3 sink with service account and region",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &v1alpha1.AWSS3{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-bucket",
					},
					Auth: &v1alpha1.Auth{
						ServiceAccountName: "aws-service-account",
					},
				},
			},
			want: nil,
		},
		{
			name: "valid AWS SQS sink with auth and region",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					SQS: &v1alpha1.AWSSQS{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-queue",
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "valid AWS SQS sink with service account and region",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					SQS: &v1alpha1.AWSSQS{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-queue",
					},
					Auth: &v1alpha1.Auth{
						ServiceAccountName: "aws-service-account",
					},
				},
			},
			want: nil,
		},
		{
			name: "multiple sinks set (invalid)",
			spec: IntegrationSinkSpec{
				Log: &Log{
					Level:       "info",
					ShowHeaders: true,
				},
				Aws: &Aws{
					S3: &v1alpha1.AWSS3{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-bucket",
					},
				},
			},
			want: apis.ErrGeneric("only one sink type can be set", "spec"),
		},
		{
			name: "multiple AWS sinks set (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &v1alpha1.AWSS3{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-bucket",
					},
					SQS: &v1alpha1.AWSSQS{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-queue",
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: apis.ErrGeneric("only one sink type can be set", "spec"),
		},
		{
			name: "AWS SQS sink without QueueNameOrArn (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					SQS: &v1alpha1.AWSSQS{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: apis.ErrMissingField("aws.sqs.arn"),
		},
		{
			name: "AWS SNS sink without TopicNameOrArn (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					SNS: &v1alpha1.AWSSNS{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: apis.ErrMissingField("aws.sns.arn"),
		},
		{
			name: "AWS Eventbridge sink without TopicNameOrArn (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					EVENTBRIDGE: &v1alpha1.AWSEventbridge{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: apis.ErrMissingField("aws.eventbridge.arn"),
		},
		{
			name: "no sink type specified (invalid)",
			spec: IntegrationSinkSpec{},
			want: apis.ErrGeneric("at least one sink type must be specified", "spec"),
		},
		{
			name: "AWS sink without auth (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &v1alpha1.AWSS3{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-bucket",
					},
				},
			},
			want: apis.ErrMissingField("aws.auth.secret.ref.name"),
		},
		{
			name: "AWS sink without auth credentials (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &v1alpha1.AWSS3{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-bucket",
					},
					Auth: &v1alpha1.Auth{},
				},
			},
			want: apis.ErrMissingField("aws.auth.secret.ref.name"),
		},
		{
			name: "AWS S3 sink without region (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &v1alpha1.AWSS3{
						Arn: "example-bucket",
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: apis.ErrMissingField("aws.s3.region"),
		},
		{
			name: "AWS Eventbridge sink without region (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					EVENTBRIDGE: &v1alpha1.AWSEventbridge{
						Arn: "example-bus",
					},
					Auth: &v1alpha1.Auth{
						Secret: &v1alpha1.Secret{
							Ref: &v1alpha1.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: apis.ErrMissingField("aws.eventbridge.region"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("IntegrationSinkSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
