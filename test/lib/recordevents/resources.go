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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/pkg/test"
)

// EventRecordPod creates a Pod that stores received events for test retrieval.
func EventRecordPod(name string, namespace string, serviceAccountName string) *corev1.Pod {
	return recordEventsPod("recordevents", name, namespace, serviceAccountName)
}

func recordEventsPod(imageName string, name string, namespace string, serviceAccountName string) *corev1.Pod {
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
					Name:  "SYSTEM_NAMESPACE",
					Value: namespace,
				}, {
					Name:  "OBSERVER",
					Value: "recorder-" + name,
				}, {
					Name:  "K8S_EVENT_SINK",
					Value: fmt.Sprintf("{\"apiVersion\": \"corev1\", \"kind\": \"Pod\", \"name\": \"%s\", \"namespace\": \"%s\"}", name, namespace),
				}},
			}},
			ServiceAccountName: serviceAccountName,
			RestartPolicy:      corev1.RestartPolicyAlways,
		},
	}
}
