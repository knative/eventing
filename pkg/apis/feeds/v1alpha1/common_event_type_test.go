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

func TestCommonEventTypeCondition_SetCondition(t *testing.T) {
	testcases := []struct {
		name  string
		types []CommonEventTypeConditionType
		want  int
	}{
		{"Simple", []CommonEventTypeConditionType{EventTypeComplete}, 1},
		{"Two", []CommonEventTypeConditionType{EventTypeComplete, EventTypeFailed}, 2},
		{"Override", []CommonEventTypeConditionType{EventTypeComplete, EventTypeComplete}, 1},
		{"Invalid", []CommonEventTypeConditionType{""}, 0},
	}

	// EventType
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "EventType", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := EventType{}
			for _, t := range tc.types {
				c := &CommonEventTypeCondition{
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
	// ClusterEventType
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "ClusterEventType", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := ClusterEventType{}
			for _, t := range tc.types {
				c := &CommonEventTypeCondition{
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

func TestCommonEventTypeCondition_RemoveCondition(t *testing.T) {
	testcases := []struct {
		name   string
		set    []CommonEventTypeConditionType
		remove []CommonEventTypeConditionType
		want   int
	}{
		{"Simple", []CommonEventTypeConditionType{EventTypeComplete}, []CommonEventTypeConditionType{EventTypeComplete}, 0},
		{"One", []CommonEventTypeConditionType{EventTypeComplete, EventTypeFailed}, []CommonEventTypeConditionType{EventTypeComplete}, 1},
		{"Missing", []CommonEventTypeConditionType{EventTypeComplete}, []CommonEventTypeConditionType{EventTypeFailed}, 1},
		{"Invalid", []CommonEventTypeConditionType{EventTypeComplete}, []CommonEventTypeConditionType{""}, 1},
	}

	// EventType
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "EventType", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := EventType{}
			for _, t := range tc.set {
				c := &CommonEventTypeCondition{
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

	// ClusterEventType
	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "ClusterEventType", tc.name)
		t.Run(testName, func(t *testing.T) {
			src := ClusterEventType{}
			for _, t := range tc.set {
				c := &CommonEventTypeCondition{
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
