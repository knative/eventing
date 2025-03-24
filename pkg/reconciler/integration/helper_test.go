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

package integration

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestGenerateEnvVarsFromStruct(t *testing.T) {
	type TestStruct struct {
		Field1 int    `json:"field1"`
		Field2 bool   `json:"field2"`
		Field3 string `json:"field3"`
	}

	prefix := "TEST_PREFIX"
	input := &TestStruct{
		Field1: 123,
		Field2: true,
		Field3: "hello",
	}

	// Expected environment variables including SSL settings
	want := []corev1.EnvVar{
		{Name: "TEST_PREFIX_FIELD1", Value: "123"},
		{Name: "TEST_PREFIX_FIELD2", Value: "true"},
		{Name: "TEST_PREFIX_FIELD3", Value: "hello"},
	}

	got := GenerateEnvVarsFromStruct(prefix, input)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("generateEnvVarsFromStruct() mismatch (-want +got):\n%s", diff)
	}
}

func TestGenerateEnvVarsFromStruct_S3WithCamelTag(t *testing.T) {
	type AWSS3 struct {
		Arn    string `json:"arn,omitempty" camel:"BUCKETNAMEORARN"`
		Region string `json:"region,omitempty"`
	}

	type AWSSQS struct {
		Arn    string `json:"arn,omitempty" camel:"QUEUENAMEORARN"`
		Region string `json:"region,omitempty"`
	}

	tests := []struct {
		name   string
		prefix string
		input  interface{}
		want   []corev1.EnvVar
	}{
		{
			name:   "S3 Source with Camel Tag",
			prefix: "CAMEL_KAMELET_AWS_S3_SOURCE",
			input: AWSS3{
				Arn:    "arn:aws:s3:::example-bucket",
				Region: "us-west-2",
			},
			want: []corev1.EnvVar{
				{Name: "CAMEL_KAMELET_AWS_S3_SOURCE_BUCKETNAMEORARN", Value: "arn:aws:s3:::example-bucket"},
				{Name: "CAMEL_KAMELET_AWS_S3_SOURCE_REGION", Value: "us-west-2"},
			},
		},
		{
			name:   "S3 Sink with Camel Tag",
			prefix: "CAMEL_KAMELET_AWS_S3_SINK",
			input: AWSS3{
				Arn:    "arn:aws:s3:::another-bucket",
				Region: "eu-central-1",
			},
			want: []corev1.EnvVar{
				{Name: "CAMEL_KAMELET_AWS_S3_SINK_BUCKETNAMEORARN", Value: "arn:aws:s3:::another-bucket"},
				{Name: "CAMEL_KAMELET_AWS_S3_SINK_REGION", Value: "eu-central-1"},
			},
		},
		{
			name:   "SQS Source with Camel Tag",
			prefix: "CAMEL_KAMELET_AWS_SQS_SOURCE",
			input: AWSSQS{
				Arn:    "arn:aws:sqs:::example-queue",
				Region: "us-east-1",
			},
			want: []corev1.EnvVar{
				{Name: "CAMEL_KAMELET_AWS_SQS_SOURCE_QUEUENAMEORARN", Value: "arn:aws:sqs:::example-queue"},
				{Name: "CAMEL_KAMELET_AWS_SQS_SOURCE_REGION", Value: "us-east-1"},
			},
		},
		{
			name:   "SQS Sink with Camel Tag",
			prefix: "CAMEL_KAMELET_AWS_SQS_SINK",
			input: AWSSQS{
				Arn:    "arn:aws:sqs:::another-queue",
				Region: "ap-southeast-1",
			},
			want: []corev1.EnvVar{
				{Name: "CAMEL_KAMELET_AWS_SQS_SINK_QUEUENAMEORARN", Value: "arn:aws:sqs:::another-queue"},
				{Name: "CAMEL_KAMELET_AWS_SQS_SINK_REGION", Value: "ap-southeast-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateEnvVarsFromStruct(tt.prefix, tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GenerateEnvVarsFromStruct() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
