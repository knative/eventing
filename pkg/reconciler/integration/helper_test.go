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
		Arn    string `json:"arn,omitempty" camel:"CAMEL_KAMELET_AWS_S3_SOURCE_BUCKETNAMEORARN"`
		Region string `json:"region,omitempty"`
	}

	prefix := "CAMEL_KAMELET_AWS_S3_SOURCE"
	input := AWSS3{
		Arn:    "arn:aws:s3:::example-bucket",
		Region: "us-west-2",
	}

	// Expected environment variables including SSL settings and camel tag for Arn
	want := []corev1.EnvVar{
		{Name: "CAMEL_KAMELET_AWS_S3_SOURCE_BUCKETNAMEORARN", Value: "arn:aws:s3:::example-bucket"},
		{Name: "CAMEL_KAMELET_AWS_S3_SOURCE_REGION", Value: "us-west-2"},
	}

	got := GenerateEnvVarsFromStruct(prefix, input)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("generateEnvVarsFromStruct_S3WithCamelTag() mismatch (-want +got):\n%s", diff)
	}
}
