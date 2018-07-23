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

package v1alpha1

import (
	"fmt"
	"testing"
)

func TestCommonEventSourceCondition_SetCondition(t *testing.T) {
	testcases := []struct {
		name  string
		types []CommonEventSourceConditionType
		want  int
	}{
		{"Simple", []CommonEventSourceConditionType{EventSourceComplete}, 1},
		{"Two", []CommonEventSourceConditionType{EventSourceComplete, EventSourceFailed}, 2},
		{"Override", []CommonEventSourceConditionType{EventSourceComplete, EventSourceComplete}, 1},
		{"Invalid", []CommonEventSourceConditionType{""}, 0},
	}

	// EventSource
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "EventSource", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := EventSource{}
			for _, t := range tc.types {
				c := &CommonEventSourceCondition{
					Type: t,
				}
				src.Status.SetCondition(c)
			}
			got := len(src.Status.Conditions)
			if tc.want != got {
				t.Fatalf("Failed to return expecected number of conditions. \nwant:\t%#v\ngot:\t%#v", tc.want, got)
			}
		})
	}
	// ClusterEventSource
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "ClusterEventSource", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := ClusterEventSource{}
			for _, t := range tc.types {
				c := &CommonEventSourceCondition{
					Type: t,
				}
				src.Status.SetCondition(c)
			}
			got := len(src.Status.Conditions)
			if tc.want != got {
				t.Fatalf("Failed to return expecected number of conditions. \nwant:\t%#v\ngot:\t%#v", tc.want, got)
			}
		})
	}
}

func TestCommonEventSourceCondition_RemoveCondition(t *testing.T) {
	testcases := []struct {
		name   string
		set    []CommonEventSourceConditionType
		remove []CommonEventSourceConditionType
		want   int
	}{
		{"Simple", []CommonEventSourceConditionType{EventSourceComplete}, []CommonEventSourceConditionType{EventSourceComplete}, 0},
		{"One", []CommonEventSourceConditionType{EventSourceComplete, EventSourceFailed}, []CommonEventSourceConditionType{EventSourceComplete}, 1},
		{"Missing", []CommonEventSourceConditionType{EventSourceComplete}, []CommonEventSourceConditionType{EventSourceFailed}, 1},
		{"Invalid", []CommonEventSourceConditionType{EventSourceComplete}, []CommonEventSourceConditionType{""}, 1},
	}

	// EventSource
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "EventSource", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := EventSource{}
			for _, t := range tc.set {
				c := &CommonEventSourceCondition{
					Type: t,
				}
				src.Status.SetCondition(c)
			}
			for _, t := range tc.remove {
				src.Status.RemoveCondition(t)
			}

			got := len(src.Status.Conditions)
			if tc.want != got {
				t.Fatalf("Failed to return expecected number of conditions. \nwant:\t%#v\ngot:\t%#v", tc.want, got)
			}
		})
	}

	// ClusterEventSource
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "ClusterEventSource", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := ClusterEventSource{}
			for _, t := range tc.set {
				c := &CommonEventSourceCondition{
					Type: t,
				}
				src.Status.SetCondition(c)
			}
			for _, t := range tc.remove {
				src.Status.RemoveCondition(t)
			}

			got := len(src.Status.Conditions)
			if tc.want != got {
				t.Fatalf("Failed to return expecected number of conditions. \nwant:\t%#v\ngot:\t%#v", tc.want, got)
			}
		})
	}
}
