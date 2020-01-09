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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	_ "knative.dev/pkg/metrics/testing"
)

func TestMakeReceiveAdapter(t *testing.T) {
	name := "source-name"
	src := &v1alpha1.ApiServerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "source-namespace",
			UID:       "1234",
		},
		Spec: v1alpha1.ApiServerSourceSpec{
			ServiceAccountName: "source-svc-acct",
			Resources: []v1alpha1.ApiServerResource{
				{
					APIVersion: "",
					Kind:       "Namespace",
				},
				{
					APIVersion: "",
					Kind:       "Pod",
					Controller: true,
				},
				{
					APIVersion: "",
					Kind:       "Pod",
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"test-key1": "test-value1"},
					},
				},
				{
					APIVersion: "",
					Kind:       "Pod",
					LabelSelector: metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "akey", Operator: "Exists"},
							{Key: "anotherkey", Operator: "DoesNotExist"},
						},
					},
				},
				{
					APIVersion: "",
					Kind:       "Pod",
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"test-key2": "test-value2"},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "akey", Operator: "Exists"},
							{Key: "anotherkey", Operator: "DoesNotExist"},
						},
					},
					ControllerSelector: metav1.OwnerReference{
						APIVersion: "foo/v1alpha1",
						Kind:       "Foo",
					},
				},
			},
		},
	}

	got := MakeReceiveAdapter(&ReceiveAdapterArgs{
		Image:  "test-image",
		Source: src,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SinkURI: "sink-uri",
	})

	one := int32(1)
	trueValue := true

	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-namespace",
			Name:      fmt.Sprintf("apiserversource-%s-1234", name),
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "sources.knative.dev/v1alpha1",
					Kind:               "ApiServerSource",
					Name:               name,
					UID:                "1234",
					Controller:         &trueValue,
					BlockOwnerDeletion: &trueValue,
				},
			},
		},
		Spec: v1.DeploymentSpec{
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
									Name:  "SINK_URI",
									Value: "sink-uri",
								}, {
									Name: "MODE",
								}, {
									Name:  "API_VERSION",
									Value: ";;;;",
								}, {
									Name:  "KIND",
									Value: "Namespace;Pod;Pod;Pod;Pod",
								}, {
									Name:  "OWNER_API_VERSION",
									Value: ";;;;foo/v1alpha1",
								}, {
									Name:  "OWNER_KIND",
									Value: ";;;;Foo",
								}, {
									Name:  "CONTROLLER",
									Value: "false,true,false,false,false",
								}, {
									Name:  "SELECTOR",
									Value: ";;test-key1=test-value1;akey,!anotherkey;akey,!anotherkey,test-key2=test-value2",
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
									Name:  "K_METRICS_CONFIG",
									Value: "",
								}, {
									Name:  "K_LOGGING_CONFIG",
									Value: "",
								},
							},
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
