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

package recordevents

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/pkg/test"
)

// EventRecordPod creates a Pod that stores received events for test retrieval.
func EventRecordPod(name string, serviceAccountName string) *corev1.Pod {
	return recordEventsPod("recordevents", name, serviceAccountName)
}

func recordEventsPod(imageName string, name string, serviceAccountName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           test.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
				Env: []corev1.EnvVar{{
					Name: "SYSTEM_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
					},
				}, {
					Name:  "OBSERVER",
					Value: "recorder-" + name,
				}, {
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
					},
				}},
			}},
			ServiceAccountName: serviceAccountName,
			RestartPolicy:      corev1.RestartPolicyAlways,
		},
	}
}
