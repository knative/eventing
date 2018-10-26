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
	channelName       = "subscribedChannel"
	resultChannelName = "toChannel"
	subscriberName    = "subscriber"
)

func getValidChannelRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:       channelName,
		Kind:       channelKind,
		APIVersion: channelAPIVersion,
	}
}

func getValidResultStrategy() *ResultStrategy {
	return &ResultStrategy{
		Target: &corev1.ObjectReference{
			Name:       resultChannelName,
			Kind:       channelKind,
			APIVersion: channelAPIVersion,
		},
	}
}

func getValidEndpointSpec() *EndpointSpec {
	return &EndpointSpec{
		TargetRef: &corev1.ObjectReference{
			Name:       subscriberName,
			Kind:       routeKind,
			APIVersion: routeAPIVersion,
		},
	}
}

type DummyImmutableType struct{}

func (d *DummyImmutableType) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
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
		got := c.Validate()
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
			Subscriber: getValidEndpointSpec(),
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
			Subscriber: getValidEndpointSpec(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channel.name")
			return fe
		}(),
	}, {
		name: "missing Subscriber and Result",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("result", "subscriber")
			fe.Details = "the Subscription must reference at least one of (result channel or a subscriber)"
			return fe
		}(),
	}, {
		name: "empty Subscriber and Result",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &EndpointSpec{},
			Result:     &ResultStrategy{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("result", "subscriber")
			fe.Details = "the Subscription must reference at least one of (result channel or a subscriber)"
			return fe
		}(),
	}, {
		name: "missing Result",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidEndpointSpec(),
		},
		want: nil,
	}, {
		name: "empty Result",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: getValidEndpointSpec(),
			Result:     &ResultStrategy{},
		},
		want: nil,
	}, {
		name: "missing Subscriber",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Result:  getValidResultStrategy(),
		},
		want: nil,
	}, {
		name: "empty Subscriber",
		c: &SubscriptionSpec{
			Channel:    getValidChannelRef(),
			Subscriber: &EndpointSpec{},
			Result:     getValidResultStrategy(),
		},
		want: nil,
	}, {
		name: "missing name in channel, and missing subscriber, result",
		c: &SubscriptionSpec{
			Channel: corev1.ObjectReference{
				Kind:       channelKind,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("result", "subscriber")
			fe.Details = "the Subscription must reference at least one of (result channel or a subscriber)"
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
		name: "missing name in Subscriber.TargetRef",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Subscriber: &EndpointSpec{
				TargetRef: &corev1.ObjectReference{
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("subscriber.targetRef.name")
			return fe
		}(),
	}, {
		name: "missing name in Result.TargetRef",
		c: &SubscriptionSpec{
			Channel: getValidChannelRef(),
			Result: &ResultStrategy{
				Target: &corev1.ObjectReference{
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("result.target.name")
			return fe
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.Validate()
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validateChannel (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSubscriptionImmutable(t *testing.T) {
	newChannel := getValidChannelRef()
	newChannel.Name = "newChannel"

	newSubscriber := getValidEndpointSpec()
	newSubscriber.TargetRef.Name = "newSubscriber"

	newResult := getValidResultStrategy()
	newResult.Target.Name = "newResultChannel"

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
				Subscriber: getValidEndpointSpec(),
			},
		},
		og:   nil,
		want: nil,
	}, {
		name: "valid, new Subscriber",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidEndpointSpec(),
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
		name: "valid, new Result",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Result:  getValidResultStrategy(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Result:  newResult,
			},
		},
		want: nil,
	}, {
		name: "valid, have Result, remove and replace with Subscriber",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Result:  getValidResultStrategy(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidEndpointSpec(),
			},
		},
		want: nil,
	}, {
		name: "valid, have Subscriber, remove and replace with Result",
		c: &Subscription{
			Spec: SubscriptionSpec{
				Channel:    getValidChannelRef(),
				Subscriber: getValidEndpointSpec(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				Channel: getValidChannelRef(),
				Result:  getValidResultStrategy(),
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
			got := test.c.CheckImmutableFields(test.og)
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
			Subscriber: getValidEndpointSpec(),
		},
	}
	og := &DummyImmutableType{}
	want := &apis.FieldError{
		Message: "The provided original was not a Subscription",
	}
	t.Run(name, func(t *testing.T) {
		got := c.CheckImmutableFields(og)
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
			fe := apis.ErrInvalidValue("", "apiVersion")
			fe.Details = "only eventing.knative.dev/v1alpha1 is allowed for apiVersion"
			return apis.ErrMissingField("apiVersion").Also(fe)
		}(),
	}, {
		name: "missing kind",
		c: corev1.ObjectReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrInvalidValue("", "kind")
			fe.Details = "only 'Channel' kind is allowed"
			return apis.ErrMissingField("kind").Also(fe)
		}(),
	}, {
		name: "invalid kind",
		c: corev1.ObjectReference{
			Name:       channelName,
			APIVersion: channelAPIVersion,
			Kind:       "subscription",
		},
		want: func() *apis.FieldError {
			fe := apis.ErrInvalidValue("subscription", "kind")
			fe.Details = "only 'Channel' kind is allowed"
			return fe
		}(),
	}, {
		name: "invalid apiVersion",
		c: corev1.ObjectReference{
			Name:       channelName,
			APIVersion: "wrongapiversion",
			Kind:       channelKind,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrInvalidValue("wrongapiversion", "apiVersion")
			fe.Details = "only eventing.knative.dev/v1alpha1 is allowed for apiVersion"
			return fe
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

func TestValidgetValidEndpointSpec(t *testing.T) {
	dnsName := "example.com"
	tests := []struct {
		name string
		e    EndpointSpec
		want *apis.FieldError
	}{{
		name: "valid targetRef",
		e:    *getValidEndpointSpec(),
		want: nil,
	}, {
		name: "valid dnsName",
		e: EndpointSpec{
			DNSName: &dnsName,
		},
		want: nil,
	}, {
		name: "both targetRef and dnsName given",
		e: EndpointSpec{
			TargetRef: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
			DNSName: &dnsName,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMultipleOneOf("targetRef", "dnsName")
			return fe
		}(),
	}, {
		name: "missing name in targetRef",
		e: EndpointSpec{
			TargetRef: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("targetRef.name")
			return fe
		}(),
	}, {
		name: "missing apiVersion in targetRef",
		e: EndpointSpec{
			TargetRef: &corev1.ObjectReference{
				Name: channelName,
				Kind: channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("targetRef.apiVersion")
			return fe
		}(),
	}, {
		name: "missing kind in targetRef",
		e: EndpointSpec{
			TargetRef: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("targetRef.kind")
			return fe
		}(),
	}, {
		name: "extra field, namespace",
		e: EndpointSpec{
			TargetRef: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "secretnamespace",
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("targetRef.Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		e: EndpointSpec{
			TargetRef: &corev1.ObjectReference{
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
			got := isValidEndpointSpec(test.e)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: isValidChannel (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestValidResultStrategy(t *testing.T) {
	tests := []struct {
		name string
		c    ResultStrategy
		want *apis.FieldError
	}{{
		name: "valid target",
		c:    *getValidResultStrategy(),
		want: nil,
	}, {
		name: "missing name in target",
		c: ResultStrategy{
			Target: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("target.name")
			return fe
		}(),
	}, {
		name: "missing apiVersion in target",
		c: ResultStrategy{
			Target: &corev1.ObjectReference{
				Name: channelName,
				Kind: channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("target.apiVersion")
			return fe
		}(),
	}, {
		name: "missing kind in target",
		c: ResultStrategy{
			Target: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("target.kind")
			return fe
		}(),
	}, {
		name: "invalid kind",
		c: ResultStrategy{
			Target: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       "subscription",
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrInvalidValue("subscription", "kind")
			fe.Details = "only 'Channel' kind is allowed"
			return fe
		}(),
	}, {
		name: "invalid apiVersion",
		c: ResultStrategy{
			Target: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: "wrongapiversion",
				Kind:       channelKind,
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrInvalidValue("wrongapiversion", "apiVersion")
			fe.Details = "only eventing.knative.dev/v1alpha1 is allowed for apiVersion"
			return fe
		}(),
	}, {
		name: "extra field, namespace",
		c: ResultStrategy{
			Target: &corev1.ObjectReference{
				Name:       channelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "secretnamespace",
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("target.Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		c: ResultStrategy{
			Target: &corev1.ObjectReference{
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
			got := isValidResultStrategy(test.c)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: isValidChannel (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
