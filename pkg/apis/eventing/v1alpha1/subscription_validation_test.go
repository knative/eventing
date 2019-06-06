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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
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

func getValidReplyStrategy() *ReplyStrategy {
	return &ReplyStrategy{
		Channel: &corev1.ObjectReference{
			Name:       replyChannelName,
			Kind:       channelKind,
			APIVersion: channelAPIVersion,
		},
	}
}

func getValidSubscriberSpec() *SubscriberSpec {
	return &SubscriberSpec{
		Ref: &corev1.ObjectReference{
			Name:       subscriberName,
			Kind:       routeKind,
			APIVersion: routeAPIVersion,
		},
	}
}

type DummyImmutableType struct{}

func (d *DummyImmutableType) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	return nil
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
			Subscriber: getValidSubscriberSpec(),
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
			Subscriber: getValidSubscriberSpec(),
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
			Subscriber: &SubscriberSpec{},
			Reply:      &ReplyStrategy{},
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
			Subscriber: getValidSubscriberSpec(),
		},
		want: nil,
	}, {
		name: "empty Reply",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidSubscriberSpec(),
			Reply:      &ReplyStrategy{},
		},
		want: nil,
	}, {
		name: "missing Subscriber",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Reply:   getValidReplyStrategy(),
		},
		want: nil,
	}, {
		name: "empty Subscriber",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &SubscriberSpec{},
			Reply:      getValidReplyStrategy(),
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
			Subscriber: &SubscriberSpec{
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
	}, {
		name: "missing name in Reply.Ref",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Reply: &ReplyStrategy{
				Channel: &corev1.ObjectReference{
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("reply.channel.name")
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

	newSubscriber := getValidSubscriberSpec()
	newSubscriber.Ref.Name = "newSubscriber"

	newReply := getValidReplyStrategy()
	newReply.Channel.Name = "newReplyChannel"

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
				Subscriber: getValidSubscriberSpec(),
			},
		},
		og:   nil,
		want: nil,
	}, {
		name: "valid, new Subscriber",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidSubscriberSpec(),
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
				Reply:   getValidReplyStrategy(),
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
				Reply:   getValidReplyStrategy(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidSubscriberSpec(),
			},
		},
		want: nil,
	}, {
		name: "valid, have Subscriber, remove and replace with Reply",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidSubscriberSpec(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Reply:   getValidReplyStrategy(),
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
			Details: `{v1alpha1.SubscriptionSpec}.Channel.Name:
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

func TestInvalidImmutableType(t *testing.T) {
	name := "invalid type"
	c := &Subscription{
		Spec: SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidSubscriberSpec(),
		},
	}
	og := &DummyImmutableType{}
	want := &apis.FieldError{
		Message: "The provided original was not a Subscription",
	}
	t.Run(name, func(t *testing.T) {
		got := c.CheckImmutableFields(context.TODO(), og)
		if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
			t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
		}
	})
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

func TestValidgetValidSubscriber(t *testing.T) {
	uri := "http://example.com"
	empty := ""
	tests := []struct {
		name string
		s    SubscriberSpec
		want *apis.FieldError
	}{{
		name: "valid ref",
		s:    *getValidSubscriberSpec(),
		want: nil,
	}, {
		name: "valid dnsName",
		s: SubscriberSpec{
			DeprecatedDNSName: &uri,
		},
		want: nil,
	}, {
		name: "both ref and dnsName given",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
			DeprecatedDNSName: &uri,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMultipleOneOf("ref", "dnsName")
			return fe
		}(),
	}, {
		name: "valid uri",
		s: SubscriberSpec{
			URI: &uri,
		},
		want: nil,
	}, {
		name: "both ref and uri given",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
			URI: &uri,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMultipleOneOf("ref", "uri")
			return fe
		}(),
	}, {
		name: "both dnsName and uri given",
		s: SubscriberSpec{
			DeprecatedDNSName: &uri,
			URI:               &uri,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMultipleOneOf("dnsName", "uri")
			return fe
		}(),
	}, {
		name: "ref, dnsName, and uri given",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
			DeprecatedDNSName: &uri,
			URI:               &uri,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMultipleOneOf("ref", "dnsName", "uri")
			return fe
		}(),
	}, {
		name: "empty ref, dnsName, and uri given",
		s: SubscriberSpec{
			Ref:               &corev1.ObjectReference{},
			DeprecatedDNSName: &empty,
			URI:               &empty,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingOneOf("ref", "dnsName", "uri")
			return fe
		}(),
	}, {
		name: "missing name in ref",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("ref.name")
			return fe
		}(),
	}, {
		name: "missing apiVersion in ref",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				Name: channelName,
				Kind: channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("ref.apiVersion")
			return fe
		}(),
	}, {
		name: "missing kind in ref",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("ref.kind")
			return fe
		}(),
	}, {
		name: "extra field, namespace",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "secretnamespace",
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("ref.Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		s: SubscriberSpec{
			Ref: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "",
			},
		},
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := IsValidSubscriberSpec(test.s)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: isValidSubscriber (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestValidReply(t *testing.T) {
	tests := []struct {
		name string
		r    ReplyStrategy
		want *apis.FieldError
	}{{
		name: "valid target",
		r:    *getValidReplyStrategy(),
		want: nil,
	}, {
		name: "missing name in target",
		r: ReplyStrategy{
			Channel: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel.name")
			return fe
		}(),
	}, {
		name: "missing apiVersion in target",
		r: ReplyStrategy{
			Channel: &corev1.ObjectReference{
				Name: channelName,
				Kind: channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel.apiVersion")
			return fe
		}(),
	}, {
		name: "missing kind in target",
		r: ReplyStrategy{
			Channel: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel.kind")
			return fe
		}(),
	}, {
		name: "extra field, namespace",
		r: ReplyStrategy{
			Channel: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "secretnamespace",
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("channel.Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		r: ReplyStrategy{
			Channel: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "",
			},
		},
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidReply(test.r)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: isValidReply (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
