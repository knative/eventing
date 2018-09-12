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
	corev1 "k8s.io/api/core/v1"
)

const (
	channelKind       = "Channel"
	channelAPIVersion = "eventing.knative.dev/v1alpha1"
	routeKind         = "Route"
	routeAPIVersion   = "serving.knative.dev/v1alpha1"
	FromChannelName   = "fromChannel"
	ToChannelName     = "toChannel"
	ProcessorName     = "processor"
)

func getValidFromRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       FromChannelName,
		Kind:       channelKind,
		APIVersion: channelAPIVersion,
	}
}

func getValidToRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       ToChannelName,
		Kind:       channelKind,
		APIVersion: channelAPIVersion,
	}
}

func getValidProcessor() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Name:       ProcessorName,
		Kind:       routeKind,
		APIVersion: routeAPIVersion,
	}
}

type DummyImmutableType struct{}

func (d *DummyImmutableType) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	return nil
}

func TestSubscriptionValidation(t *testing.T) {
	name := "empty from"
	c := &Subscription{

		Spec: SubscriptionSpec{
			From: &corev1.ObjectReference{},
		},
	}
	want := &apis.FieldError{
		Paths:   []string{"spec.from"},
		Message: "missing field(s)",
		Details: "the Subscription must reference a from channel",
	}

	t.Run(name, func(t *testing.T) {
		got := c.Validate()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
		}
	})

}

func TestSubscriptionSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		c    *SubscriptionSpec
		want *apis.FieldError
	}{{
		name: "valid",
		c: &SubscriptionSpec{
			From:      getValidFromRef(),
			Processor: getValidProcessor(),
		},
		want: nil,
	}, {
		name: "valid with arguments",
		c: &SubscriptionSpec{
			From:      getValidFromRef(),
			Processor: getValidProcessor(),
			Arguments: &[]Argument{{Name: "foo", Value: "bar"}},
		},
		want: nil,
	}, {
		name: "empty from",
		c: &SubscriptionSpec{
			From: &corev1.ObjectReference{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("from")
			fe.Details = "the Subscription must reference a from channel"
			return fe
		}(),
	}, {
		name: "missing processor and to",
		c: &SubscriptionSpec{
			From: getValidFromRef(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("to", "processor")
			fe.Details = "the Subscription must reference at least one of (to channel or a processor)"
			return fe
		}(),
	}, {
		name: "empty processor and to",
		c: &SubscriptionSpec{
			From:      getValidFromRef(),
			Processor: &corev1.ObjectReference{},
			To:        &corev1.ObjectReference{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("to", "processor")
			fe.Details = "the Subscription must reference at least one of (to channel or a processor)"
			return fe
		}(),
	}, {
		name: "missing to",
		c: &SubscriptionSpec{
			From:      getValidFromRef(),
			Processor: getValidProcessor(),
		},
		want: nil,
	}, {
		name: "empty to",
		c: &SubscriptionSpec{
			From:      getValidFromRef(),
			Processor: getValidProcessor(),
			To:        &corev1.ObjectReference{},
		},
		want: nil,
	}, {
		name: "missing processor",
		c: &SubscriptionSpec{
			From: getValidFromRef(),
			To:   getValidToRef(),
		},
		want: nil,
	}, {
		name: "empty processor",
		c: &SubscriptionSpec{
			From:      getValidFromRef(),
			Processor: &corev1.ObjectReference{},
			To:        getValidToRef(),
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
	newFrom := getValidFromRef()
	newFrom.Name = "newFromChannel"

	newProcessor := getValidProcessor()
	newProcessor.Name = "newProcessor"

	tests := []struct {
		name string
		c    *Subscription
		og   *Subscription
		want *apis.FieldError
	}{{
		name: "valid",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
			},
		},
		want: nil,
	}, {
		name: "new nil is ok",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From:      getValidFromRef(),
				Processor: getValidProcessor(),
			},
		},
		og:   nil,
		want: nil,
	}, {
		name: "valid, new processor",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From:      getValidFromRef(),
				Processor: getValidProcessor(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From:      getValidFromRef(),
				Processor: newProcessor,
			},
		},
		want: nil,
	}, {
		name: "from changed",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From: newFrom,
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.SubscriptionSpec}.From.Name:
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

func TestInvalidImmutableType(t *testing.T) {
	name := "invalid type"
	c := &Subscription{
		Spec: SubscriptionSpec{
			From:      getValidFromRef(),
			Processor: getValidProcessor(),
		},
	}
	og := &DummyImmutableType{}
	want := &apis.FieldError{
		Message: "The provided original was not a Subscription",
	}
	t.Run(name, func(t *testing.T) {
		got := c.CheckImmutableFields(og)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
		}
	})
}
