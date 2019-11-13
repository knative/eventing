/*
Copyright 2019 The Knative Authors

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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
)

const (
	saName    = "my-test-sa"
	imageName = "my-test-image"
)

func TestNewDispatcher(t *testing.T) {
	os.Setenv(system.NamespaceEnvKey, "knative-testing")
	imc := &v1alpha1.InMemoryChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imcName,
			Namespace: testNS,
		},
	}
	args := DispatcherArgs{
		ServiceAccountName: saName,
		DispatcherName:     serviceName,
		Image:              imageName,
	}

	replicas := int32(1)
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      serviceName,
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: dispatcherLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: dispatcherLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: saName,
					Containers: []corev1.Container{
						{
							Name:  "dispatcher",
							Image: imageName,
							Env: []corev1.EnvVar{{
								Name:  system.NamespaceEnvKey,
								Value: "knative-testing",
							}, {
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							}, {
								Name:  "SCOPE",
								Value: "namespace",
							}, {
								Name:  "METRICS_DOMAIN",
								Value: "knative.dev/inmemorychannel-dispatcher",
							}, {
								Name:  "CONFIG_OBSERVABILITY_NAME",
								Value: "config-observability",
							}, {
								Name:  "CONFIG_LOGGING_NAME",
								Value: "config-logging",
							}},
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
						},
					},
				},
			},
		},
	}

	got := MakeDispatcher(imc, args)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected condition (-want, +got) = %v", diff)
	}
}
