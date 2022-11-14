/*
Copyright 2020 The Knative Authors. All Rights Reserved.
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

package v1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/feature"
)

const (
	channelKind         = "MyChannel"
	channelAPIVersion   = "messaging.knative.dev/v1"
	routeKind           = "Route"
	routeAPIVersion     = "serving.knative.dev/v1"
	channelName         = "subscribedChannel"
	replyChannelName    = "toChannel"
	subscriberName      = "subscriber"
	namespace           = "namespace"
	retryCount          = int32(3)
	backoffPolicy       = eventingduckv1.BackoffPolicyExponential
	backoffDelayValid   = "PT0.5S"
	backoffDelayInvalid = "invalid-delay"
)

func getValidChannelRef() duckv1.KReference {
	return duckv1.KReference{
		Name:       channelName,
		Kind:       channelKind,
		APIVersion: channelAPIVersion,
	}
}

func getValidReply() *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			Namespace:  namespace,
			Name:       replyChannelName,
			Kind:       channelKind,
			APIVersion: channelAPIVersion,
		},
	}
}

func getValidDestination() *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			Namespace:  namespace,
			Name:       subscriberName,
			Kind:       routeKind,
			APIVersion: routeAPIVersion,
		},
	}
}

func getDelivery(delay string) *eventingduckv1.DeliverySpec {
	retry := retryCount
	policy := backoffPolicy
	return &eventingduckv1.DeliverySpec{
		Retry:         &retry,
		BackoffPolicy: &policy,
		BackoffDelay:  &delay,
	}
}

func TestSubscriptionValidation(t *testing.T) {
	name := "empty channel"
	c := &Subscription{
		Spec: SubscriptionSpec{
			Channel: duckv1.KReference{},
		},
	}
	want := &apis.FieldError{
		Paths:   []string{"spec.channel"},
		Message: "missing field(s)",
		Details: "the Subscription must reference a channel",
	}

	t.Run(name, func(t *testing.T) {
		got := c.Validate(context.TODO())
		if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
			t.Error("Subscription.Validate (-want, +got) =", diff)
		}
	})

}

func TestSubscriptionSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		c    *SubscriptionSpec
		want *apis.FieldError
	}{{
		name: "valid minimal spec",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
		},
		want: nil,
	}, {
		name: "valid with reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Reply:      getValidReply(),
		},
		want: nil,
	}, {
		name: "valid with delivery",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Delivery:   getDelivery(backoffDelayValid),
		},
		want: nil,
	}, {
		name: "empty Channel",
		c: &SubscriptionSpec{
			Channel: duckv1.KReference{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel")
			fe.Details = "the Subscription must reference a channel"
			return fe
		}(),
	}, {
		name: "missing name in Channel",
		c: &SubscriptionSpec{
			Channel: duckv1.KReference{
				Kind:       channelKind,
				APIVersion: channelAPIVersion,
			},
			Subscriber: getValidDestination(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel.name")
			return fe
		}(),
	}, {
		name: "missing Subscriber and Reply",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "empty Subscriber and Reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &duckv1.Destination{},
			Reply:      &duckv1.Destination{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "empty Reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Reply:      &duckv1.Destination{},
		},
		want: nil,
	}, {
		name: "missing Subscriber",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Reply:   getValidReply(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "empty Subscriber",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &duckv1.Destination{},
			Reply:      getValidReply(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "missing name in channel, and missing subscriber",
		c: &SubscriptionSpec{
			Channel: duckv1.KReference{
				Kind:       channelKind,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return apis.ErrMissingField("channel.name").Also(fe)
		}(),
	}, {
		name: "empty",
		c:    &SubscriptionSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel")
			fe.Details = "the Subscription must reference a channel"
			return fe
		}(),
	}, {
		name: "missing name in Subscriber.Ref",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Subscriber: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Namespace:  namespace,
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: apis.ErrMissingField("subscriber.ref.name"),
	}, {
		name: "empty name in Subscriber.Ref",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Reply: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Namespace:  namespace,
					Name:       "",
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: apis.ErrMissingField("reply.ref.name"),
	}, {
		name: "invalid Delivery",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Delivery:   getDelivery(backoffDelayInvalid),
		},
		want: apis.ErrInvalidValue(backoffDelayInvalid, "delivery.backoffDelay"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validateChannel (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSubscriptionSpecValidationWithKRefGroupFeatureEnabled(t *testing.T) {
	tests := []struct {
		name string
		c    *SubscriptionSpec
		want *apis.FieldError
	}{{
		name: "valid",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
		},
		want: nil,
	}, {
		name: "valid with reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Reply:      getValidReply(),
		},
		want: nil,
	}, {
		name: "empty Channel",
		c: &SubscriptionSpec{
			Channel: duckv1.KReference{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel")
			fe.Details = "the Subscription must reference a channel"
			return fe
		}(),
	}, {
		name: "missing name in Channel",
		c: &SubscriptionSpec{
			Channel: duckv1.KReference{
				Kind:       channelKind,
				APIVersion: channelAPIVersion,
			},
			Subscriber: getValidDestination(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel.name")
			return fe
		}(),
	}, {
		name: "missing Subscriber and Reply",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "empty Subscriber and Reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &duckv1.Destination{},
			Reply:      &duckv1.Destination{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "missing Reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
		},
		want: nil,
	}, {
		name: "empty Reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Reply:      &duckv1.Destination{},
		},
		want: nil,
	}, {
		name: "missing Subscriber",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Reply:   getValidReply(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "empty Subscriber",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &duckv1.Destination{},
			Reply:      getValidReply(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return fe
		}(),
	}, {
		name: "missing name in channel, and missing subscriber",
		c: &SubscriptionSpec{
			Channel: duckv1.KReference{
				Kind:       channelKind,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber")
			fe.Details = "the Subscription must reference a subscriber"
			return apis.ErrMissingField("channel.name").Also(fe)
		}(),
	}, {
		name: "empty",
		c:    &SubscriptionSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel")
			fe.Details = "the Subscription must reference a channel"
			return fe
		}(),
	}, {
		name: "missing name in Subscriber.Ref",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Subscriber: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Namespace:  namespace,
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: apis.ErrMissingField("subscriber.ref.name"),
	}, {
		name: "missing name in Subscriber.Ref",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidDestination(),
			Reply: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Namespace:  namespace,
					Name:       "",
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: apis.ErrMissingField("reply.ref.name"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := feature.ToContext(context.TODO(), feature.Flags{
				feature.KReferenceGroup: feature.Allowed,
			})
			got := test.c.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validateChannel (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSubscriptionImmutable(t *testing.T) {
	newChannel := getValidChannelRef()
	newChannel.Name = "newChannel"

	newSubscriber := getValidDestination()
	newSubscriber.Ref.Name = "newSubscriber"

	newReply := getValidReply()
	newReply.Ref.Name = "newReplyChannel"

	tests := []struct {
		name string
		c    *Subscription
		og   *Subscription
		want *apis.FieldError
	}{{
		name: "valid",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
			},
		},
		want: nil,
	}, {
		name: "new nil is ok",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
			},
		},
		og:   nil,
		want: nil,
	}, {
		name: "valid, new Subscriber",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: newSubscriber,
			},
		},
		want: nil,
	}, {
		name: "valid, new Reply",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
				Reply:      getValidReply(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Reply:   newReply,
			},
		},
		want: nil,
	}, {
		name: "valid, have Subscriber, remove and replace with Reply",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Reply:   getValidReply(),
			},
		},
		want: nil,
	}, {
		name: "valid, change delivery spec",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
				Delivery:   &eventingduckv1.DeliverySpec{Retry: pointer.Int32(2)},
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
				Delivery:   &eventingduckv1.DeliverySpec{Retry: pointer.Int32(3)},
			},
		},
		want: nil,
	}, {
		name: "Channel changed",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidDestination(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    newChannel,
				Subscriber: getValidDestination(),
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1.SubscriptionSpec}.Channel.Name:
	-: "newChannel"
	+: "subscribedChannel"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = apis.WithinUpdate(ctx, test.og)
			got := test.c.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("CheckImmutableFields (-want, +got) =", diff)
			}
		})
	}
}

func TestValidChannel(t *testing.T) {
	tests := []struct {
		name string
		c    duckv1.KReference
		want *apis.FieldError
	}{{
		name: "valid",
		c: duckv1.KReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
		},
		want: nil,
	}, {
		name: "missing name",
		c: duckv1.KReference{
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("name")
			return fe
		}(),
	}, {
		name: "missing apiVersion",
		c: duckv1.KReference{
			Name: channelName,
			Kind: channelKind,
		},
		want: func() *apis.FieldError {
			return apis.ErrMissingField("apiVersion")
		}(),
	}, {
		name: "extra field, namespace",
		c: duckv1.KReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
			Namespace:  "secretnamespace",
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		c: duckv1.KReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
			Namespace:  "",
		},
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidChannel(context.TODO(), test.c)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("isValidChannel (-want, +got) =", diff)
			}
		})
	}
}
