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
	"knative.dev/eventing/pkg/apis/common"
	"testing"
)

func TestIntegrationSink_GetStatus(t *testing.T) {
	r := &IntegrationSink{
		Status: IntegrationSinkStatus{},
	}
	if got, want := r.GetStatus(), &r.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}

func TestIntegrationSink_GetGroupVersionKind(t *testing.T) {
	src := &IntegrationSink{}
	gvk := src.GetGroupVersionKind()

	if gvk.Kind != "IntegrationSink" {
		t.Errorf("Should be IntegrationSink.")
	}
}

func TestLog(t *testing.T) {
	log := Log{
		Level:       "info",
		ShowHeaders: true,
	}

	if log.Level != "info" {
		t.Errorf("Log.Level = %v, want 'info'", log.Level)
	}
	if log.ShowHeaders != true {
		t.Errorf("Log.ShowHeaders = %v, want 'false'", log.ShowHeaders)
	}
}

func TestAWS(t *testing.T) {
	s3 := common.AWSS3{
		AWSCommon: common.AWSCommon{
			Region: "eu-north-1",
		},
		BucketNameOrArn: "example-bucket",
	}

	if s3.Region != "eu-north-1" {
		t.Errorf("AWSS3.Region = %v, want 'eu-north-1'", s3.Region)
	}

	sqs := common.AWSSQS{
		AWSCommon: common.AWSCommon{
			Region: "eu-north-1",
		},
		QueueNameOrArn: "example-queue",
	}

	if sqs.Region != "eu-north-1" {
		t.Errorf("AWSSQS.Region = %v, want 'eu-north-1'", sqs.Region)
	}

	ddbStreams := common.AWSDDBStreams{
		AWSCommon: common.AWSCommon{
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
	auth := common.Auth{
		Secret: &common.Secret{
			Ref: &common.SecretReference{
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
