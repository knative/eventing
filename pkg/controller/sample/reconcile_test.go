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

package sample

import (
	"testing"

	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func getUnlabeledRS() *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "test",
			Labels:    map[string]string{},
		},
		Spec: appsv1.ReplicaSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "testcontainer",
						},
					},
				},
			},
		},
	}
}

func labelRS(rs *appsv1.ReplicaSet) *appsv1.ReplicaSet {
	rs.Labels["hello"] = "world"
	return rs
}

func TestLabel(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name:         "TestLabel",
			InitialState: []runtime.Object{getUnlabeledRS()},
			ReconcileKey: "test/test",
			WantPresent:  []runtime.Object{labelRS(getUnlabeledRS())},
		},
		{
			Name:         "TestAlreadyLabeled",
			InitialState: []runtime.Object{labelRS(getUnlabeledRS())},
			ReconcileKey: "test/test",
			WantPresent:  []runtime.Object{labelRS(getUnlabeledRS())},
		},
	}

	for _, tc := range testCases {
		r := &reconciler{
			client: tc.GetClient(),
		}
		t.Run(tc.Name, tc.Runner(t, r, r.client))
	}
}
