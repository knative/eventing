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

package resources

import (
	"fmt"
	"testing"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	testName      = "test-integrationsource"
	testNamespace = "test-namespace"
	testUID       = "test-uid"
)

func TestNewContainerSource(t *testing.T) {
	source := &v1alpha1.IntegrationSource{

		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			UID:       testUID,
		},
		Spec: v1alpha1.IntegrationSourceSpec{
			Timer: &v1alpha1.Timer{
				Period:      1000,
				Message:     "test-message",
				ContentType: "text/plain",
				RepeatCount: 0,
			},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					URI: apis.HTTP("http://test-sink"),
				},
			},
		},
	}

	want := &sourcesv1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-containersource", testName),
			Namespace: testNamespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
		},
		Spec: sourcesv1.ContainerSourceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "source",
							Image:           "gcr.io/knative-nightly/timer-source:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "CAMEL_KNATIVE_CLIENT_SSL_ENABLED", Value: "true"},
								{Name: "CAMEL_KNATIVE_CLIENT_SSL_CERT_PATH", Value: "/knative-custom-certs/knative-eventing-bundle.pem"},
								{Name: "CAMEL_KAMELET_TIMER_SOURCE_PERIOD", Value: "1000"},
								{Name: "CAMEL_KAMELET_TIMER_SOURCE_MESSAGE", Value: "test-message"},
								{Name: "CAMEL_KAMELET_TIMER_SOURCE_CONTENTTYPE", Value: "text/plain"},
								{Name: "CAMEL_KAMELET_TIMER_SOURCE_REPEATCOUNT", Value: "0"},
							},
						},
					},
				},
			},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					URI: apis.HTTP("http://test-sink"),
				},
			},
		},
	}

	got := NewContainerSource(source)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NewContainerSource() mismatch (-want +got):\n%s", diff)
	}
}

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

	got := generateEnvVarsFromStruct(prefix, input)

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

	got := generateEnvVarsFromStruct(prefix, input)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("generateEnvVarsFromStruct_S3WithCamelTag() mismatch (-want +got):\n%s", diff)
	}
}
