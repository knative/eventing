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

package duck

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestCheckResourceReady(t *testing.T) {
	// We will Get this object, so use it in the objects for all test cases.
	om := metav1.ObjectMeta{
		Namespace: "my-ns",
		Name:      "my-name",
	}
	// Add the scheme for any object in a test case. Failing to do will cause the test to panic.
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	v1.AddToScheme(scheme)
	testCases := map[string]struct {
		obj       runtime.Object
		isReady   bool
		expectErr bool
	}{
		"pod succeeded": {
			obj: &corev1.Pod{
				ObjectMeta: om,
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			isReady: true,
		},
		"pod running": {
			obj: &corev1.Pod{
				ObjectMeta: om,
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			isReady: true,
		},
		"pod pending": {
			obj: &corev1.Pod{
				ObjectMeta: om,
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			isReady: false,
		},
		"pod unspecified": {
			obj: &corev1.Pod{
				ObjectMeta: om,
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
			},
			isReady: false,
		},
		"broker running": {
			obj: &v1.Broker{
				ObjectMeta: om,
				TypeMeta: metav1.TypeMeta{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
				},
				Status: v1.BrokerStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   "Ready",
								Status: "True",
							},
						},
					},
				},
			},
			isReady: true,
		},
		"broker unknown": {
			obj: &v1.Broker{
				ObjectMeta: om,
				TypeMeta: metav1.TypeMeta{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
				},
				Status: v1.BrokerStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   "Ready",
								Status: "Unknown",
							},
						},
					},
				},
			},
			isReady: false,
		},
		"broker not running": {
			obj: &v1.Broker{
				ObjectMeta: om,
				TypeMeta: metav1.TypeMeta{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
				},
				Status: v1.BrokerStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   "Ready",
								Status: "False",
							},
						},
					},
				},
			},
			isReady: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			dc := fake.NewSimpleDynamicClient(scheme, tc.obj)
			apiVersion, kind := tc.obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
			typeMeta := metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       kind,
			}
			metaResource := resources.NewMetaResource(om.Name, om.Namespace, &typeMeta)
			ready, err := checkResourceReady(dc, metaResource)
			if ready != tc.isReady {
				t.Errorf("Unexpected readiness. Expected %v, actually %v", tc.isReady, ready)
			}
			if tc.expectErr != (err != nil) {
				t.Errorf("Unexpected error. Expected error = %v, actually %v", tc.expectErr, err)
			}
		})
	}
}
