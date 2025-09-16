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

func TestIntegrationSourceSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		spec IntegrationSourceSpec
		want *apis.FieldError
	}{
		{
			name: "valid timer source",
			spec: IntegrationSourceSpec{
				Timer: &Timer{
					Period:      1000,
					Message:     "test message",
					ContentType: "text/plain",
				},
			},
			want: nil,
		},
		{
			name: "valid AWS S3 source with auth and region",
			spec: IntegrationSourceSpec{
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
			name: "valid AWS S3 source with service account and region",
			spec: IntegrationSourceSpec{
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
			name: "valid AWS SQS source with auth and region",
			spec: IntegrationSourceSpec{
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
			name: "valid AWS SQS source with service account and region",
			spec: IntegrationSourceSpec{
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
			name: "valid AWS DDBStreams source with auth and region",
			spec: IntegrationSourceSpec{
				Aws: &Aws{
					DDBStreams: &v1alpha1.AWSDDBStreams{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Table: "example-table",
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
			name: "valid AWS DDBStreams source with service account and region",
			spec: IntegrationSourceSpec{
				Aws: &Aws{
					DDBStreams: &v1alpha1.AWSDDBStreams{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Table: "example-table",
					},
					Auth: &v1alpha1.Auth{
						ServiceAccountName: "aws-service-account",
					},
				},
			},
			want: nil,
		},
		{
			name: "multiple sources set (invalid)",
			spec: IntegrationSourceSpec{
				Timer: &Timer{
					Period:      1000,
					Message:     "test message",
					ContentType: "text/plain",
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
			want: apis.ErrGeneric("only one source type can be set", "spec"),
		},
		{
			name: "multiple AWS sources set (invalid)",
			spec: IntegrationSourceSpec{
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
			want: apis.ErrGeneric("only one source type can be set", "spec"),
		},
		{
			name: "AWS SQS source without QueueNameOrArn (invalid)",
			spec: IntegrationSourceSpec{
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
			name: "AWS SQS source without Auth (invalid)",
			spec: IntegrationSourceSpec{
				Aws: &Aws{
					SQS: &v1alpha1.AWSSQS{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-queue",
					},
				},
			},
			want: apis.ErrMissingField("aws.auth.secret.ref.name"),
		},
		{
			name: "AWS SQS source without Auth credentials (invalid)",
			spec: IntegrationSourceSpec{
				Aws: &Aws{
					SQS: &v1alpha1.AWSSQS{
						AWSCommon: v1alpha1.AWSCommon{
							Region: "us-east-1",
						},
						Arn: "example-queue",
					},
					Auth: &v1alpha1.Auth{},
				},
			},
			want: apis.ErrMissingField("aws.auth.secret.ref.name"),
		},
		{
			name: "AWS DDBStreams source without Table (invalid)",
			spec: IntegrationSourceSpec{
				Aws: &Aws{
					DDBStreams: &v1alpha1.AWSDDBStreams{
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
			want: apis.ErrMissingField("aws.ddb-streams.table"),
		},
		{
			name: "no source type specified (invalid)",
			spec: IntegrationSourceSpec{},
			want: apis.ErrGeneric("at least one source type must be specified", "spec"),
		},
		{
			name: "AWS source without auth (invalid)",
			spec: IntegrationSourceSpec{
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
			name: "AWS S3 source without region (invalid)",
			spec: IntegrationSourceSpec{
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("IntegrationSourceSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
