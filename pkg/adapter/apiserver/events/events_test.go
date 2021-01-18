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

package events_test

import (
	"fmt"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"knative.dev/eventing/pkg/adapter/apiserver/events"
)

var contentType = "application/json"

func simplePod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func simpleSubject(name, namespace string) *string {
	subject := fmt.Sprintf("/apis/v1/namespaces/%s/pods/%s", namespace, name)
	return &subject
}

func TestMakeAddEvent(t *testing.T) {
	testCases := map[string]struct {
		obj    interface{}
		source string

		want     *cloudevents.Event
		wantData string
		wantErr  string
	}{
		"nil object": {
			source:  "unit-test",
			want:    nil,
			wantErr: "resource can not be nil",
		},
		"simple pod": {
			source: "unit-test",
			obj:    simplePod("unit", "test"),
			want: &cloudevents.Event{
				Context: cloudevents.EventContextV1{
					Type:            "dev.knative.apiserver.resource.add",
					Source:          *cloudevents.ParseURIRef("unit-test"),
					Subject:         simpleSubject("unit", "test"),
					DataContentType: &contentType,
					Extensions: map[string]interface{}{
						"kind":      "Pod",
						"name":      "unit",
						"namespace": "test",
					},
				}.AsV1(),
			},
			wantData: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"unit","namespace":"test"}}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, got, err := events.MakeAddEvent(tc.source, tc.obj, false)
			validate(t, got, err, tc.want, tc.wantData, tc.wantErr)
		})
	}
}

func TestMakeUpdateEvent(t *testing.T) {
	testCases := map[string]struct {
		obj    interface{}
		source string

		want     *cloudevents.Event
		wantData string
		wantErr  string
	}{
		"nil object": {
			source:  "unit-test",
			want:    nil,
			wantErr: "new resource can not be nil",
		},
		"simple pod": {
			source: "unit-test",
			obj:    simplePod("unit", "test"),
			want: &cloudevents.Event{
				Context: cloudevents.EventContextV1{
					Type:            "dev.knative.apiserver.resource.update",
					Source:          *cloudevents.ParseURIRef("unit-test"),
					Subject:         simpleSubject("unit", "test"),
					DataContentType: &contentType,
					Extensions: map[string]interface{}{
						"kind":      "Pod",
						"name":      "unit",
						"namespace": "test",
					},
				}.AsV1(),
			},
			wantData: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"unit","namespace":"test"}}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, got, err := events.MakeUpdateEvent(tc.source, tc.obj, false)
			validate(t, got, err, tc.want, tc.wantData, tc.wantErr)
		})
	}
}

func TestMakeDeleteEvent(t *testing.T) {
	testCases := map[string]struct {
		obj    interface{}
		source string

		want     *cloudevents.Event
		wantData string
		wantErr  string
	}{
		"nil object": {
			source:  "unit-test",
			want:    nil,
			wantErr: "resource can not be nil",
		},
		"simple pod": {
			source: "unit-test",
			obj:    simplePod("unit", "test"),
			want: &cloudevents.Event{
				Context: cloudevents.EventContextV1{
					Type:            "dev.knative.apiserver.resource.delete",
					Source:          *cloudevents.ParseURIRef("unit-test"),
					Subject:         simpleSubject("unit", "test"),
					DataContentType: &contentType,
					Extensions: map[string]interface{}{
						"kind":      "Pod",
						"name":      "unit",
						"namespace": "test",
					},
				}.AsV1(),
			},
			wantData: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"unit","namespace":"test"}}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, got, err := events.MakeDeleteEvent(tc.source, tc.obj, false)
			validate(t, got, err, tc.want, tc.wantData, tc.wantErr)
		})
	}
}

func TestMakeAddRefEvent(t *testing.T) {
	testCases := map[string]struct {
		obj    interface{}
		source string

		want     *cloudevents.Event
		wantData string
		wantErr  string
	}{
		"nil object": {
			source:  "unit-test",
			want:    nil,
			wantErr: "resource can not be nil",
		},
		"simple pod": {
			source: "unit-test",
			obj:    simplePod("unit", "test"),
			want: &cloudevents.Event{
				Context: cloudevents.EventContextV1{
					Type:            "dev.knative.apiserver.ref.add",
					Source:          *cloudevents.ParseURIRef("unit-test"),
					Subject:         simpleSubject("unit", "test"),
					DataContentType: &contentType,
					Extensions: map[string]interface{}{
						"kind":      "Pod",
						"name":      "unit",
						"namespace": "test",
					},
				}.AsV1(),
			},
			wantData: `{"kind":"Pod","namespace":"test","name":"unit","apiVersion":"v1"}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, got, err := events.MakeAddEvent(tc.source, tc.obj, true)
			validate(t, got, err, tc.want, tc.wantData, tc.wantErr)
		})
	}
}

func TestMakeUpdateRefEvent(t *testing.T) {
	testCases := map[string]struct {
		obj    interface{}
		source string

		want     *cloudevents.Event
		wantData string
		wantErr  string
	}{
		"nil object": {
			source:  "unit-test",
			want:    nil,
			wantErr: "new resource can not be nil",
		},
		"simple pod": {
			source: "unit-test",
			obj:    simplePod("unit", "test"),
			want: &cloudevents.Event{
				Context: cloudevents.EventContextV1{
					Type:            "dev.knative.apiserver.ref.update",
					Source:          *cloudevents.ParseURIRef("unit-test"),
					Subject:         simpleSubject("unit", "test"),
					DataContentType: &contentType,
					Extensions: map[string]interface{}{
						"kind":      "Pod",
						"name":      "unit",
						"namespace": "test",
					},
				}.AsV1(),
			},
			wantData: `{"kind":"Pod","namespace":"test","name":"unit","apiVersion":"v1"}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, got, err := events.MakeUpdateEvent(tc.source, tc.obj, true)
			validate(t, got, err, tc.want, tc.wantData, tc.wantErr)
		})
	}
}

func TestMakeDeleteRefEvent(t *testing.T) {
	testCases := map[string]struct {
		obj    interface{}
		source string

		want     *cloudevents.Event
		wantData string
		wantErr  string
	}{
		"nil object": {
			source:  "unit-test",
			want:    nil,
			wantErr: "resource can not be nil",
		},
		"simple pod": {
			source: "unit-test",
			obj:    simplePod("unit", "test"),
			want: &cloudevents.Event{
				Context: cloudevents.EventContextV1{
					Type:            "dev.knative.apiserver.ref.delete",
					Source:          *cloudevents.ParseURIRef("unit-test"),
					Subject:         simpleSubject("unit", "test"),
					DataContentType: &contentType,
					Extensions: map[string]interface{}{
						"kind":      "Pod",
						"name":      "unit",
						"namespace": "test",
					},
				}.AsV1(),
			},
			wantData: `{"kind":"Pod","namespace":"test","name":"unit","apiVersion":"v1"}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, got, err := events.MakeDeleteEvent(tc.source, tc.obj, true)
			validate(t, got, err, tc.want, tc.wantData, tc.wantErr)
		})
	}
}

func validate(t *testing.T, got cloudevents.Event, err error, want *cloudevents.Event, wantData, wantErr string) {
	if wantErr != "" || err != nil {
		var gotErr string
		if err != nil {
			gotErr = err.Error()
		}
		if !strings.Contains(wantErr, gotErr) {
			diff := cmp.Diff(wantErr, gotErr)
			t.Error("unexpected error (-want, +got) =", diff)
		}
		return
	}

	if diff := cmp.Diff(want, &got, cmpopts.IgnoreFields(cloudevents.Event{}, "DataBase64", "DataEncoded")); diff != "" {
		t.Error("unexpected event diff (-want, +got) =", diff)
	}

	gotData := string(got.Data())
	if diff := cmp.Diff(wantData, gotData); diff != "" {
		t.Error("unexpected data diff (-want, +got) =", diff)
	}
}
