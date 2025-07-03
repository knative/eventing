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

	"knative.dev/eventing/pkg/reconciler/integration"

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
	const timerImage = "quay.io/timer-image"
	t.Setenv("INTEGRATION_SOURCE_TIMER_IMAGE", timerImage)

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
			Template: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "unexpected_container",
							Image: "undesired_image",
						},
					},
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
			Labels: integration.Labels(source.Name),
		},
		Spec: sourcesv1.ContainerSourceSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: integration.Labels(source.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "source",
							Image:           timerImage,
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

	got := NewContainerSource(source, false)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NewContainerSource() mismatch (-want +got):\n%s", diff)
	}
}
