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
	"knative.dev/eventing/pkg/apis/common"
	"testing"

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
					S3: &common.AWSS3{
						AWSCommon: common.AWSCommon{
							Region: "us-east-1",
						},
						BucketNameOrArn: "example-bucket",
					},
					Auth: &common.Auth{
						Secret: &common.Secret{
							Ref: &common.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "valid AWS SQS sink with auth and region",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					SQS: &common.AWSSQS{
						AWSCommon: common.AWSCommon{
							Region: "us-east-1",
						},
						QueueNameOrArn: "example-queue",
					},
					Auth: &common.Auth{
						Secret: &common.Secret{
							Ref: &common.SecretReference{
								Name: "aws-secret",
							},
						},
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
					S3: &common.AWSS3{
						AWSCommon: common.AWSCommon{
							Region: "us-east-1",
						},
						BucketNameOrArn: "example-bucket",
					},
				},
			},
			want: apis.ErrGeneric("only one sink type can be set", "spec"),
		},
		{
			name: "multiple AWS sinks set (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &common.AWSS3{
						AWSCommon: common.AWSCommon{
							Region: "us-east-1",
						},
						BucketNameOrArn: "example-bucket",
					},
					SQS: &common.AWSSQS{
						AWSCommon: common.AWSCommon{
							Region: "us-east-1",
						},
						QueueNameOrArn: "example-queue",
					},
					Auth: &common.Auth{
						Secret: &common.Secret{
							Ref: &common.SecretReference{
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
					SQS: &common.AWSSQS{
						AWSCommon: common.AWSCommon{
							Region: "us-east-1",
						},
					},
					Auth: &common.Auth{
						Secret: &common.Secret{
							Ref: &common.SecretReference{
								Name: "aws-secret",
							},
						},
					},
				},
			},
			want: apis.ErrMissingField("aws.sqs.queueNameOrArn"),
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
					S3: &common.AWSS3{
						AWSCommon: common.AWSCommon{
							Region: "us-east-1",
						},
						BucketNameOrArn: "example-bucket",
					},
				},
			},
			want: apis.ErrMissingField("aws.auth.secret.ref.name"),
		},
		{
			name: "AWS S3 sink without region (invalid)",
			spec: IntegrationSinkSpec{
				Aws: &Aws{
					S3: &common.AWSS3{
						BucketNameOrArn: "example-bucket",
					},
					Auth: &common.Auth{
						Secret: &common.Secret{
							Ref: &common.SecretReference{
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
				t.Errorf("IntegrationSinkSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
