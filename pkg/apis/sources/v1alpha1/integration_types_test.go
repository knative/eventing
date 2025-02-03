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
	"testing"

	"knative.dev/eventing/pkg/apis/common/integration/v1alpha1"
)

func TestIntegrationSource_GetStatus(t *testing.T) {
	r := &IntegrationSource{
		Status: IntegrationSourceStatus{},
	}
	if got, want := r.GetStatus(), &r.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}

func TestIntegrationSource_GetGroupVersionKind(t *testing.T) {
	src := &IntegrationSource{}
	gvk := src.GetGroupVersionKind()

	if gvk.Kind != "IntegrationSource" {
		t.Errorf("Should be IntegrationSource.")
	}
}

func TestTimer(t *testing.T) {
	timer := Timer{
		Period:  1000,
		Message: "test message",
	}

	if timer.Period != 1000 {
		t.Errorf("Timer.Period = %v, want '1000'", timer.Period)
	}
	if timer.Message != "test message" {
		t.Errorf("Timer.Message = %v, want 'test message'", timer.Message)
	}
}

func TestAWS(t *testing.T) {
	s3 := v1alpha1.AWSS3{
		AWSCommon: v1alpha1.AWSCommon{
			Region: "eu-north-1",
		},
		Arn: "example-bucket",
	}

	if s3.Region != "eu-north-1" {
		t.Errorf("AWSS3.Region = %v, want 'eu-north-1'", s3.Region)
	}

	sqs := v1alpha1.AWSSQS{
		AWSCommon: v1alpha1.AWSCommon{
			Region: "eu-north-1",
		},
		Arn: "example-queue",
	}

	if sqs.Region != "eu-north-1" {
		t.Errorf("AWSSQS.Region = %v, want 'eu-north-1'", sqs.Region)
	}

	ddbStreams := v1alpha1.AWSDDBStreams{
		AWSCommon: v1alpha1.AWSCommon{
			Region: "eu-north-1",
		},
		Table: "example-table",
	}

	if ddbStreams.Region != "eu-north-1" {
		t.Errorf("AWSDDBStreams.Region = %v, want 'eu-north-1'", ddbStreams.Region)
	}
}

// TestAuthFieldAccess tests the HasAuth method and field access in Auth struct
func TestAuthFieldAccess(t *testing.T) {
	auth := v1alpha1.Auth{
		Secret: &v1alpha1.Secret{
			Ref: &v1alpha1.SecretReference{
				Name: "aws-secret",
			},
		},
		AccessKey: "access-key",
		SecretKey: "secret-key",
	}

	if !auth.HasAuth() {
		t.Error("Auth.HasAuth() = false, want true")
	}

	if auth.Secret.Ref.Name != "aws-secret" {
		t.Errorf("Auth.Secret.Ref.Name = %v, want 'aws-secret'", auth.Secret.Ref.Name)
	}
}
