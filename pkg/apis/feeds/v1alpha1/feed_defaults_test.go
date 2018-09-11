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
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestFeedDefaults(t *testing.T) {
	tests := []struct {
		name string
		f    *Feed
		want *Feed
	}{{
		name: "valid (no change)",
		f: &Feed{
			Spec: FeedSpec{
				Trigger: EventTrigger{
					EventType: "foo",
					Resource:  "baz",
				},
				ServiceAccountName: "bar",
			},
		},
		want: &Feed{
			Spec: FeedSpec{
				Trigger: EventTrigger{
					EventType: "foo",
					Resource:  "baz",
				},
				ServiceAccountName: "bar",
			},
		},
	}, {
		name: "valid (default service account)",
		f: &Feed{
			Spec: FeedSpec{
				Trigger: EventTrigger{
					EventType: "foo",
					Resource:  "baz",
				},
			},
		},
		want: &Feed{
			Spec: FeedSpec{
				Trigger: EventTrigger{
					EventType: "foo",
					Resource:  "baz",
				},
				ServiceAccountName: "default",
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.f.DeepCopy()
			got.SetDefaults()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("defaultFeed (-want, +got) = %v", diff)
			}
		})
	}
}
