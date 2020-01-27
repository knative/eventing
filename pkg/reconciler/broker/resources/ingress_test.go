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

package resources

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/service"
)

func TestMakeIngressServiceArgs(t *testing.T) {
	_ = os.Setenv("SYSTEM_NAMESPACE", "my-system-ns")
	b := &eventingv1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "my-ns",
		},
	}

	want := &service.Args{
		ServiceMeta: metav1.ObjectMeta{
			Namespace: "my-ns",
			Name:      "default-broker",
			Labels: map[string]string{
				"eventing.knative.dev/broker":     "default",
				"eventing.knative.dev/brokerRole": "ingress",
			},
		},
		DeployMeta: metav1.ObjectMeta{
			Namespace: "my-ns",
			Name:      "default-broker-ingress",
			Labels: map[string]string{
				"eventing.knative.dev/broker":     "default",
				"eventing.knative.dev/brokerRole": "ingress",
			},
		},
		PodSpec: corev1.PodSpec{
			ServiceAccountName: "my-serviceaccount",
			Containers: []corev1.Container{
				{
					Name:  "ingress",
					Image: "image.example.com/ingress",
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
							},
						},
						InitialDelaySeconds: 5,
						PeriodSeconds:       2,
						FailureThreshold:    3,
						TimeoutSeconds:      10,
						SuccessThreshold:    1,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "SYSTEM_NAMESPACE",
							Value: "my-system-ns",
						},
						{
							Name:  "NAMESPACE",
							Value: "my-ns",
						},
						{
							Name:  "POD_NAME",
							Value: "default-broker-ingress",
						},
						{
							Name:  "CONTAINER_NAME",
							Value: "ingress",
						},
						{
							Name:  "FILTER",
							Value: "",
						},
						{
							Name:  "CHANNEL",
							Value: "chan.example.com",
						},
						{
							Name:  "BROKER",
							Value: "default",
						},
						{
							Name:  "METRICS_DOMAIN",
							Value: "knative.dev/internal/eventing",
						},
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8080,
						},
					},
				},
			},
		},
	}

	got := MakeIngressServiceArgs(&IngressArgs{
		Broker:             b,
		Image:              "image.example.com/ingress",
		ServiceAccountName: "my-serviceaccount",
		ChannelAddress:     "chan.example.com",
	})

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected filter service arguments (-want, +got): %s", diff)
	}
}
