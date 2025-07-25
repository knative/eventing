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

	"k8s.io/utils/pointer"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/kncloudevents"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/config"
)

const (
	saName    = "my-test-sa"
	imageName = "my-test-image"
)

func TestNewDispatcher(t *testing.T) {
	os.Setenv(system.NamespaceEnvKey, "knative-testing")

	args := DispatcherArgs{
		EventDispatcherConfig: config.EventDispatcherConfig{
			ConnectionArgs: kncloudevents.ConnectionArgs{
				MaxIdleConns:        2000,
				MaxIdleConnsPerHost: 200,
			},
		},
		ServiceAccountName:  saName,
		DispatcherName:      serviceName,
		DispatcherNamespace: testNS,
		Image:               imageName,
	}

	replicas := int32(1)
	want := &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployments",
		},
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
					EnableServiceLinks: ptr.Bool(false),
					Containers: []corev1.Container{
						{
							Name:  "dispatcher",
							Image: imageName,
							Env: []corev1.EnvVar{{
								Name:  system.NamespaceEnvKey,
								Value: "knative-testing",
							}, {
								Name:  "METRICS_DOMAIN",
								Value: "knative.dev/inmemorychannel-dispatcher",
							}, {
								Name:  "CONFIG_OBSERVABILITY_NAME",
								Value: o11yconfigmap.Name(),
							}, {
								Name:  "CONFIG_LOGGING_NAME",
								Value: logging.ConfigMapName(),
							}, {
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							}, {
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							}, {
								Name:  "CONTAINER_NAME",
								Value: "dispatcher",
							}, {
								Name:  "MAX_IDLE_CONNS",
								Value: "2000",
							}, {
								Name:  "MAX_IDLE_CONNS_PER_HOST",
								Value: "200",
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("125m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2200m"),
									corev1.ResourceMemory: resource.MustParse("2048Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:    pointer.Int64(1000),
								RunAsGroup:   pointer.Int64(1000),
								RunAsNonRoot: pointer.Bool(true),
							},
						},
					},
				},
			},
		},
	}

	got := MakeDispatcher(args)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected condition (-want, +got) =", diff)
	}
}
