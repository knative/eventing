/*
Copyright 2018 The Knative Authors. All Rights Reserved.
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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
)

func TestSubscriptionSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		c    *SubscriptionSpec
		want *apis.FieldError
	}{{
		name: "valid",
		c: &SubscriptionSpec{
			From:      "fromChannel",
			Processor: "processor",
		},
		want: nil,
	}, {
		name: "valid with arguments",
		c: &SubscriptionSpec{
			From:      "fromChannel",
			Processor: "processor",
			Arguments: &[]Argument{{Name: "foo", Value: "bar"}},
		},
		want: nil,
	}, {
		name: "missing processor and to",
		c: &SubscriptionSpec{
			From: "fromChannel",
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("to", "processor")
			fe.Details = "the Subscription must reference a to channel or a processor"
			return fe
		}(),
	}, {
		name: "missing to",
		c: &SubscriptionSpec{
			From: "fromChannel",
			To:   "toChannel",
		},
		want: nil,
	}, {
		name: "missing processor",
		c: &SubscriptionSpec{
			From: "fromChannel",
			To:   "toChannel",
		},
		want: nil,
	}, {
		name: "empty",
		c:    &SubscriptionSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("from")
			fe.Details = "the Subscription must reference a from channel"
			return fe
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.Validate()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("validateFrom (-want, +got) = %v", diff)
			}
		})
	}
}

func TestSubscriptionImmutable(t *testing.T) {
	tests := []struct {
		name string
		c    *Subscription
		og   *Subscription
		want *apis.FieldError
	}{{
		name: "valid",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From: "foo",
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From: "foo",
			},
		},
		want: nil,
	}, {
		name: "valid, new processor",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From:      "foo",
				Processor: "newProcessor",
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From:      "foo",
				Processor: "processor",
			},
		},
		want: nil,
	}, {
		name: "from changed",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From: "fromChannel",
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From: "newFromChannel",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.SubscriptionSpec}.From:
	-: "newFromChannel"
	+: "fromChannel"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.CheckImmutableFields(test.og)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}
