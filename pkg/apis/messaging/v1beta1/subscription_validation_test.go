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

package v1beta1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	channelKind       = "MyChannel"
	channelAPIVersion = "eventing.knative.dev/v1alpha1"
	routeKind         = "Route"
	routeAPIVersion   = "serving.knative.dev/v1alpha1"
	channelName       = "subscribedChannel"
	replyChannelName  = "toChannel"
	subscriberName    = "subscriber"
)

func getValidChannelRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:       channelName,
		Kind:       channelKind,
		APIVersion: channelAPIVersion,
	}
}

func getValidReply() *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &corev1.ObjectReference{
			Name:       replyChannelName,
			Kind:       channelKind,
			APIVersion: channelAPIVersion,
		},
	}
}

func getValidDestination() *duckv1.Destination {
	return &duckv1.Destination{
		Ref: &corev1.ObjectReference{
			Name:       subscriberName,
			Kind:       routeKind,
			APIVersion: routeAPIVersion,
		},
	}
}

func TestSubscriptionValidation(t *testing.T) {
	name := "empty channel"
	c := &Subscription{
		Spec: SubscriptionSpec{
			Channel: corev1.ObjectReference{},
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
			t.Errorf("Subscription.Validate (-want, +got) = %v", diff)
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
			Channel: corev1.ObjectReference{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel")
			fe.Details = "the Subscription must reference a channel"
			return fe
		}(),
	}, {
		name: "missing name in Channel",
		c: &SubscriptionSpec{
			Channel: corev1.ObjectReference{
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
			fe := apis.ErrMissingField("reply", "subscriber")
			fe.Details = "the Subscription must reference at least one of (reply or a subscriber)"
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
			fe := apis.ErrMissingField("reply", "subscriber")
			fe.Details = "the Subscription must reference at least one of (reply or a subscriber)"
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
		want: nil,
	}, {
		name: "empty Subscriber",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &duckv1.Destination{},
			Reply:      getValidReply(),
		},
		want: nil,
	}, {
		name: "missing name in channel, and missing subscriber, reply",
		c: &SubscriptionSpec{
			Channel: corev1.ObjectReference{
				Kind:       channelKind,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("reply", "subscriber")
			fe.Details = "the Subscription must reference at least one of (reply or a subscriber)"
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
				Ref: &corev1.ObjectReference{
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber.ref.name")
			return fe
		}(),
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
				Channel: getValidChannelRef(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
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
				Channel: getValidChannelRef(),
				Reply:   getValidReply(),
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
		name: "valid, have Reply, remove and replace with Subscriber",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Reply:   getValidReply(),
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
		name: "Channel changed",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: newChannel,
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1beta1.SubscriptionSpec}.Channel.Name:
	-: "newChannel"
	+: "subscribedChannel"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.CheckImmutableFields(context.TODO(), test.og)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidChannel(t *testing.T) {
	tests := []struct {
		name string
		c    corev1.ObjectReference
		want *apis.FieldError
	}{{
		name: "valid",
		c: corev1.ObjectReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
		},
		want: nil,
	}, {
		name: "missing name",
		c: corev1.ObjectReference{
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("name")
			return fe
		}(),
	}, {
		name: "missing apiVersion",
		c: corev1.ObjectReference{
			Name: channelName,
			Kind: channelKind,
		},
		want: func() *apis.FieldError {
			return apis.ErrMissingField("apiVersion")
		}(),
	}, {
		name: "extra field, namespace",
		c: corev1.ObjectReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
			Namespace:  "secretnamespace",
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}, {
		name: "extra field, namespace and resourceVersion",
		c: corev1.ObjectReference{
			Name:            channelName,
			APIVersion:      channelAPIVersion,
			Kind:            channelKind,
			Namespace:       "secretnamespace",
			ResourceVersion: "myresourceversion",
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("Namespace", "ResourceVersion")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		c: corev1.ObjectReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
			Namespace:  "",
		},
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidChannel(test.c)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("isValidChannel (-want, +got) = %v", diff)
			}
		})
	}
}
