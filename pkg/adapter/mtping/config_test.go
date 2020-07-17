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

package mtping

import (
	"testing"

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

func TestProject(t *testing.T) {
	testCases := map[string]struct {
		source   sourcesv1alpha2.PingSource
		expected PingConfig
	}{
		"TestAddRunRemoveSchedule": {
			source: sourcesv1alpha2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns"},

				Spec: sourcesv1alpha2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: nil,
					},
					Schedule: "* * * * ?",
					JsonData: "some data",
				},
				Status: sourcesv1alpha2.PingSourceStatus{
					duckv1.SourceStatus{
						SinkURI: &apis.URL{
							Host: "asink",
						},
					},
				},
			},
			expected: PingConfig{
				ObjectReference: corev1.ObjectReference{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Schedule: "* * * * ?",
				JsonData: "some data",
				SinkURI:  "//asink",
			}},
		"TestAddRunRemoveScheduleWithExtensionOverride": {
			source: sourcesv1alpha2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns"},
				Spec: sourcesv1alpha2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{},
						CloudEventOverrides: &duckv1.CloudEventOverrides{
							Extensions: map[string]string{"1": "one", "2": "two"},
						},
					},
					Schedule: "* * * * ?",
					JsonData: "some data",
				},
				Status: sourcesv1alpha2.PingSourceStatus{
					duckv1.SourceStatus{
						SinkURI: &apis.URL{Host: "anothersink"},
					},
				},
			},
			expected: PingConfig{
				ObjectReference: corev1.ObjectReference{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Schedule:   "* * * * ?",
				JsonData:   "some data",
				Extensions: map[string]string{"1": "one", "2": "two"},
				SinkURI:    "//anothersink",
			}},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got := Project(&tc.source)
			if diff := cmp.Diff(&tc.expected, got); diff != "" {
				t.Errorf("unexpected projection (-want, +got) = %v", diff)
			}
		})
	}
}
