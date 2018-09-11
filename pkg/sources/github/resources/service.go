/*
Copyright 2018 The Knative Authors

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
)

func MakeService(namespace, name, image, serviceAccount, target, githubSecret, githubSecretKey string) *v1alpha1.Service {
	labels := map[string]string{
		"receive-adapter": "github",
	}
	return &v1alpha1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.ServiceSpec{
			RunLatest: &v1alpha1.RunLatestType{
				Configuration: v1alpha1.ConfigurationSpec{
					RevisionTemplate: v1alpha1.RevisionTemplateSpec{
						Spec: v1alpha1.RevisionSpec{
							ServiceAccountName: serviceAccount,
							Container: corev1.Container{
								Image: image,
								Env: []corev1.EnvVar{
									{
										Name: "GITHUB_SECRET",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: githubSecret,
												},
												Key: githubSecretKey,
											},
										},
									},
									{
										Name:  "TARGET",
										Value: target,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
