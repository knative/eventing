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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	"testing"
)

func TestFeedSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		f    *FeedSpec
		want *apis.FieldError
	}{{
		name: "valid namespaced",
		f: &FeedSpec{
			Trigger: EventTrigger{
				EventType: "foo",
			},
		},
	}, {
		name: "valid cluster",
		f: &FeedSpec{
			Trigger: EventTrigger{
				ClusterEventType: "foo",
			},
		},
	}, {
		name: "empty",
		f:    &FeedSpec{},
		want: apis.ErrMissingOneOf("eventType", "clusterEventType").ViaField("trigger"),
	}, {
		name: "empty trigger",
		f:    &FeedSpec{Trigger: EventTrigger{}},
		want: apis.ErrMissingOneOf("eventType", "clusterEventType").ViaField("trigger"),
	}, {
		name: "mutually exclusive missing",
		f: &FeedSpec{
			ServiceAccountName: "Sue",
		},
		want: apis.ErrMissingOneOf("eventType", "clusterEventType").ViaField("trigger"),
	}, {
		name: "mutually exclusive both",
		f: &FeedSpec{
			Trigger: EventTrigger{
				EventType:        "foo",
				ClusterEventType: "bar",
			},
		},
		want: apis.ErrMultipleOneOf("eventType", "clusterEventType").ViaField("trigger"),
	}, {
		name: "valid dns name",
		f: &FeedSpec{
			Trigger: EventTrigger{
				EventType: "foo",
			},
			Action: FeedAction{
				DNSName: "valid.com",
			},
		},
	}, {
		name: "invalidvalid dns name",
		f: &FeedSpec{
			Trigger: EventTrigger{
				EventType: "foo",
			},
			Action: FeedAction{
				DNSName: "inv@lid.com",
			},
		},
		want: &apis.FieldError{
			Message: `invalid value "inv@lid.com"`,
			Paths:   []string{"action.dnsName"},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.f.Validate()
			ignoreArguments := cmpopts.IgnoreFields(apis.FieldError{}, "Details")
			if diff := cmp.Diff(test.want, got, ignoreArguments); diff != "" {
				t.Errorf("validateFeed (-want, +got) = %v", diff)
			}
		})
	}
}
