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

func TestFeedCondition_GetConditionNotFound(t *testing.T) {
	feed := Feed{}
	if feed.Status.GetCondition(FeedConditionReady) != nil {
		t.Fatalf("Got a non-nil for non-existent conditiontype")
	}
}

func TestFeedCondition_GetCondition(t *testing.T) {
	testcases := []struct {
		name  string
		types []FeedConditionType
		get   FeedConditionType
		want  FeedConditionType
	}{
		{"FeedConditionReady", []FeedConditionType{FeedConditionReady}, FeedConditionReady, FeedConditionReady},
	}

	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Feed", tc.name)
		t.Run(testName, func(t *testing.T) {
			feed := Feed{}
			for _, t := range tc.types {
				c := &FeedCondition{Type: t}
				feed.Status.SetCondition(c)
			}

			if want, got := tc.want, feed.Status.GetCondition(tc.get).Type; want != got {
				t.Fatalf("Failed to get expected condition. \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}

func TestFeedCondition_SetCondition(t *testing.T) {
	testcases := []struct {
		name  string
		types []FeedConditionType
		want  int
	}{
		{"One", []FeedConditionType{FeedConditionReady}, 1},
		{"Replace", []FeedConditionType{FeedConditionReady, FeedConditionReady}, 1},
		{"Invalid", []FeedConditionType{""}, 0},
	}

	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Feed", tc.name)
		t.Run(testName, func(t *testing.T) {
			feed := Feed{}
			for _, t := range tc.types {
				c := &FeedCondition{Type: t}
				feed.Status.SetCondition(c)
			}
			if want, got := tc.want, len(feed.Status.Conditions); want != got {
				t.Fatalf("Failed to return expected number of conditions. \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}

func TestFeedCondition_RemoveCondition(t *testing.T) {
	testcases := []struct {
		name   string
		set    []FeedConditionType
		remove []FeedConditionType
		want   int
	}{
		{"RemoveOne", []FeedConditionType{FeedConditionReady}, []FeedConditionType{FeedConditionReady}, 0},
		{"RemoveNonExistent", []FeedConditionType{FeedConditionReady}, []FeedConditionType{"notthere"}, 1},
		{"Invalid", []FeedConditionType{FeedConditionReady}, []FeedConditionType{""}, 1},
	}

	for _, tc := range testcases {
		testName := fmt.Sprintf("%s - %s", "Feed", tc.name)
		t.Run(testName, func(t *testing.T) {
			feed := Feed{}
			for _, t := range tc.set {
				c := &FeedCondition{Type: t}
				feed.Status.SetCondition(c)
			}
			for _, t := range tc.remove {
				feed.Status.RemoveCondition(t)
			}

			if want, got := tc.want, len(feed.Status.Conditions); want != got {
				t.Fatalf("Failed to return expected number of conditions. \nwant:\t%#v\ngot:\t%#v", want, got)
			}
		})
	}
}
