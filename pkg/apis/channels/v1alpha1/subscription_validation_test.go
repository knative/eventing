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
			Channel:    "bar",
			Subscriber: "foo",
		},
		want: nil,
	}, {
		name: "valid with arguments",
		c: &SubscriptionSpec{
			Channel:    "bar",
			Subscriber: "foo",
			Arguments:  &[]Argument{{Name: "foo", Value: "bar"}},
		},
		want: nil,
	}, {
		name: "missing subscriber",
		c: &SubscriptionSpec{
			Channel: "foo",
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a Subscriber"
			return fe
		}(),
	}, {
		name: "empty",
		c:    &SubscriptionSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel")
			fe.Details = "the Subscription must reference a Channel"
			return fe
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.Validate()
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("validateChannel (-want, +got) = %v", diff)
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
				Channel: "foo",
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: "foo",
			},
		},
		want: nil,
	}, {
		name: "valid, new subscriber",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    "foo",
				Subscriber: "bar",
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    "foo",
				Subscriber: "baz",
			},
		},
		want: nil,
	}, {
		name: "channel changed",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel: "foo",
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: "bar",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.SubscriptionSpec}.Channel:
	-: "bar"
	+: "foo"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.CheckImmutableFields(test.og)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}
