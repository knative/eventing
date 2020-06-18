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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/eventing/pkg/reconciler/source"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	_ "knative.dev/pkg/metrics/testing"
	_ "knative.dev/pkg/system/testing"
)

func TestMakeReceiveAdapters(t *testing.T) {
	name := "source-name"
	one := int32(1)
	trueValue := true

	src := &v1alpha2.ApiServerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "source-namespace",
			UID:       "1234",
		},
		Spec: v1alpha2.ApiServerSourceSpec{
			Resources: []v1alpha2.APIVersionKindSelector{{
				APIVersion: "",
				Kind:       "Namespace",
			}, {
				APIVersion: "batch/v1",
				Kind:       "Job",
			}, {
				APIVersion: "",
				Kind:       "Pod",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"test-key1": "test-value1"},
				},
			}},
			ResourceOwner: &v1alpha2.APIVersionKind{
				APIVersion: "custom/v1",
				Kind:       "Parent",
			},
			EventMode:          "Resource",
			ServiceAccountName: "source-svc-acct",
		},
	}

	want := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-namespace",
			Name:      kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", name), string(src.UID)),
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "sources.knative.dev/v1alpha2",
					Kind:               "ApiServerSource",
					Name:               name,
					UID:                "1234",
					Controller:         &trueValue,
					BlockOwnerDeletion: &trueValue,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
			},
			Replicas: &one,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "source-svc-acct",
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: "test-image",
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
							Env: []corev1.EnvVar{
								{
									Name:  "K_SINK",
									Value: "sink-uri",
								}, {
									Name:  "K_SOURCE_CONFIG",
									Value: `{"namespace":"source-namespace","resources":[{"gvr":{"Group":"","Version":"","Resource":"namespaces"}},{"gvr":{"Group":"batch","Version":"v1","Resource":"jobs"}},{"gvr":{"Group":"","Version":"","Resource":"pods"},"selector":"test-key1=test-value1"}],"owner":{"apiVersion":"custom/v1","kind":"Parent"},"mode":"Resource"}`,
								}, {
									Name:  "SYSTEM_NAMESPACE",
									Value: "knative-testing",
								}, {
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								}, {
									Name:  "NAME",
									Value: name,
								}, {
									Name:  "METRICS_DOMAIN",
									Value: "knative.dev/eventing",
								}, {
									Name:  source.EnvLoggingCfg,
									Value: "",
								}, {
									Name:  source.EnvMetricsCfg,
									Value: "",
								}, {
									Name:  source.EnvTracingCfg,
									Value: "",
								},
							},
						},
					},
				},
			},
		},
	}

	ceSrc := src.DeepCopy()
	ceSrc.Spec.CloudEventOverrides = &duckv1.CloudEventOverrides{Extensions: map[string]string{"1": "one"}}
	ceWant := want.DeepCopy()
	ceWant.Spec.Template.Spec.Containers[0].Env = append(ceWant.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "K_CE_OVERRIDES",
		Value: `{"1":"one"}`,
	})

	testCases := map[string]struct {
		want *appsv1.Deployment
		src  *v1alpha2.ApiServerSource
	}{
		"TestMakeReceiveAdapter": {

			want: want,
			src:  src,
		}, "TestMakeReceiveAdapterWithExtensionOverride": {
			src:  ceSrc,
			want: ceWant,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			got, _ := MakeReceiveAdapter(&ReceiveAdapterArgs{
				Image:  "test-image",
				Source: tc.src,
				Labels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
				SinkURI: "sink-uri",
				Configs: &source.EmptyVarsGenerator{},
			})

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected deploy (-want, +got) = %v", diff)
			}

		})
	}
}
