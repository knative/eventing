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
	channelAPIVersion = "channels.knative.dev/v1alpha1"
	routeKind         = "Route"
	routeAPIVersion   = "serving.knative.dev/v1alpha1"
	fromChannelName   = "fromChannel"
	resultChannelName = "toChannel"
	callName          = "call"
)

func getValidFromRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:       fromChannelName,
		Kind:       channelKind,
		APIVersion: channelAPIVersion,
	}
}

func getValidResultRef() *ResultStrategy {
	return &ResultStrategy{
		Target: &corev1.ObjectReference{
			Name:       resultChannelName,
			Kind:       channelKind,
			APIVersion: channelAPIVersion,
		},
	}
}

func getValidCall() *Callable {
	return &Callable{
		Target: &corev1.ObjectReference{
			Name:       callName,
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
	name := "empty from"
	c := &Subscription{
		Spec: SubscriptionSpec{
			From: corev1.ObjectReference{},
		},
	}
	want := (&apis.FieldError{
		Paths:   []string{"spec.from"},
		Message: "missing field(s)",
		Details: "the Subscription must reference a from channel",
	}).Error()

	t.Run(name, func(t *testing.T) {
		got := c.Validate().Error()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Subscription.Validate (-want, +got) = %v", diff)
		}
	})

}

func TestSubscriptionSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		c    *SubscriptionSpec
		want string
	}{{
		name: "valid",
		c: &SubscriptionSpec{
			From: getValidFromRef(),
			Call: getValidCall(),
		},
		want: "",
	}, {
		name: "empty From",
		c: &SubscriptionSpec{
			From: corev1.ObjectReference{},
		},
		want: func() string {
			fe := apis.ErrMissingField("from")
			fe.Details = "the Subscription must reference a from channel"
			return fe.Error()
		}(),
	}, {
		name: "missing name in From",
		c: &SubscriptionSpec{
			From: corev1.ObjectReference{
				Kind:       channelKind,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("from.name")
			return fe.Error()
		}(),
	}, {
		name: "missing Call and Result",
		c: &SubscriptionSpec{
			From: getValidFromRef(),
		},
		want: func() string {
			fe := apis.ErrMissingField("result", "call")
			fe.Details = "the Subscription must reference at least one of (result channel or a call)"
			return fe.Error()
		}(),
	}, {
		name: "empty Call and Result",
		c: &SubscriptionSpec{
			From:   getValidFromRef(),
			Call:   &Callable{},
			Result: &ResultStrategy{},
		},
		want: func() string {
			fe := apis.ErrMissingField("result", "call")
			fe.Details = "the Subscription must reference at least one of (result channel or a call)"
			return fe.Error()
		}(),
	}, {
		name: "missing Result",
		c: &SubscriptionSpec{
			From: getValidFromRef(),
			Call: getValidCall(),
		},
		want: "",
	}, {
		name: "empty Result",
		c: &SubscriptionSpec{
			From:   getValidFromRef(),
			Call:   getValidCall(),
			Result: &ResultStrategy{},
		},
		want: "",
	}, {
		name: "missing Call",
		c: &SubscriptionSpec{
			From:   getValidFromRef(),
			Result: getValidResultRef(),
		},
		want: "",
	}, {
		name: "empty Call",
		c: &SubscriptionSpec{
			From:   getValidFromRef(),
			Call:   &Callable{},
			Result: getValidResultRef(),
		},
		want: "",
	}, {
		name: "empty",
		c:    &SubscriptionSpec{},
		want: func() string {
			fe := apis.ErrMissingField("from")
			fe.Details = "the Subscription must reference a from channel"
			return fe.Error()
		}(),
	}, {
		name: "missing name in Call.Target",
		c: &SubscriptionSpec{
			From: getValidFromRef(),
			Call: &Callable{
				Target: &corev1.ObjectReference{
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("call.target.name")
			return fe.Error()
		}(),
	}, {
		name: "missing name in Result.Target",
		c: &SubscriptionSpec{
			From: getValidFromRef(),
			Result: &ResultStrategy{
				Target: &corev1.ObjectReference{
					Kind:       channelKind,
					APIVersion: channelAPIVersion,
				},
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("result.target.name")
			return fe.Error()
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.Validate().Error()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: validateFrom (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSubscriptionImmutable(t *testing.T) {
	newFrom := getValidFromRef()
	newFrom.Name = "newFromChannel"

	newCall := getValidCall()
	newCall.Target.Name = "newCall"

	newResult := getValidResultRef()
	newResult.Target.Name = "newResultChannel"

	tests := []struct {
		name string
		c    *Subscription
		og   *Subscription
		want string
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
		want: "",
	}, {
		name: "new nil is ok",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
				Call: getValidCall(),
			},
		},
		og:   nil,
		want: "",
	}, {
		name: "valid, new Call",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
				Call: getValidCall(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
				Call: newCall,
			},
		},
		want: "",
	}, {
		name: "valid, new Result",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From:   getValidFromRef(),
				Result: getValidResultRef(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From:   getValidFromRef(),
				Result: newResult,
			},
		},
		want: "",
	}, {
		name: "valid, have Result, remove and replace with Call",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From:   getValidFromRef(),
				Result: getValidResultRef(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
				Call: getValidCall(),
			},
		},
		want: "",
	}, {
		name: "valid, have Call, remove and replace with Result",
		c: &Subscription{
			Spec: SubscriptionSpec{
				From: getValidFromRef(),
				Call: getValidCall(),
			},
		},
		og: &Subscription{
			Spec: SubscriptionSpec{
				From:   getValidFromRef(),
				Result: getValidResultRef(),
			},
		},
		want: "",
	}, {
		name: "From changed",
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
		want: (&apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.SubscriptionSpec}.From.Name:
	-: "newFromChannel"
	+: "fromChannel"
`,
		}).Error(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.CheckImmutableFields(test.og).Error()
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
			From: getValidFromRef(),
			Call: getValidCall(),
		},
	}
	og := &DummyImmutableType{}
	want := (&apis.FieldError{
		Message: "The provided original was not a Subscription",
	}).Error()
	t.Run(name, func(t *testing.T) {
		got := c.CheckImmutableFields(og).Error()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
		}
	})
}

func TestValidFrom(t *testing.T) {
	tests := []struct {
		name string
		c    corev1.ObjectReference
		want string
	}{{
		name: "valid",
		c: corev1.ObjectReference{
			Name:       fromChannelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
		},
		want: "",
	}, {
		name: "missing name",
		c: corev1.ObjectReference{
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
		},
		want: func() string {
			fe := apis.ErrMissingField("name")
			return fe.Error()
		}(),
	}, {
		name: "missing apiVersion",
		c: corev1.ObjectReference{
			Name: fromChannelName,
			Kind: channelKind,
		},
		want: func() string {
			fe := apis.ErrMissingField("apiVersion")
			return fe.Error()
		}(),
	}, {
		name: "missing kind",
		c: corev1.ObjectReference{
			Name:       fromChannelName,
			APIVersion: channelAPIVersion,
		},
		want: func() string {
			fe := apis.ErrMissingField("kind")
			return fe.Error()
		}(),
	}, {
		name: "invalid kind",
		c: corev1.ObjectReference{
			Name:       fromChannelName,
			APIVersion: channelAPIVersion,
			Kind:       "subscription",
		},
		want: func() string {
			fe := apis.ErrInvalidValue("subscription", "kind")
			fe.Details = "only 'Channel' kind is allowed"
			return fe.Error()
		}(),
	}, {
		name: "invalid apiVersion",
		c: corev1.ObjectReference{
			Name:       fromChannelName,
			APIVersion: "wrongapiversion",
			Kind:       channelKind,
		},
		want: func() string {
			fe := apis.ErrInvalidValue("wrongapiversion", "apiVersion")
			fe.Details = "only channels.knative.dev/v1alpha1 is allowed for apiVersion"
			return fe.Error()
		}(),
	}, {
		name: "extra field, namespace",
		c: corev1.ObjectReference{
			Name:       fromChannelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
			Namespace:  "secretnamespace",
		},
		want: func() string {
			fe := apis.ErrDisallowedFields("Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe.Error()
		}(),
	}, {
		name: "extra field, namespace and resourceVersion",
		c: corev1.ObjectReference{
			Name:            fromChannelName,
			APIVersion:      channelAPIVersion,
			Kind:            channelKind,
			Namespace:       "secretnamespace",
			ResourceVersion: "myresourceversion",
		},
		want: func() string {
			fe := apis.ErrDisallowedFields("Namespace", "ResourceVersion")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe.Error()
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		c: corev1.ObjectReference{
			Name:       fromChannelName,
			APIVersion: channelAPIVersion,
			Kind:       channelKind,
			Namespace:  "",
		},
		want: "",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidFrom(test.c).Error()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("isValidFrom (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidCallable(t *testing.T) {
	targetURI := "http://example.com"
	tests := []struct {
		name string
		c    Callable
		want string
	}{{
		name: "valid target",
		c:    *getValidCall(),
		want: "",
	}, {
		name: "valid targetURI",
		c: Callable{
			TargetURI: &targetURI,
		},
		want: "",
	}, {
		name: "both target and targetURI given",
		c: Callable{
			Target: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
			TargetURI: &targetURI,
		},
		want: func() string {
			fe := apis.ErrMultipleOneOf("target", "targetURI")
			return fe.Error()
		}(),
	}, {
		name: "missing name in target",
		c: Callable{
			Target: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("target.name")
			return fe.Error()
		}(),
	}, {
		name: "missing apiVersion in target",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name: fromChannelName,
				Kind: channelKind,
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("target.apiVersion")
			return fe.Error()
		}(),
	}, {
		name: "missing kind in target",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name:       fromChannelName,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("target.kind")
			return fe.Error()
		}(),
	}, {
		name: "extra field, namespace",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name:       fromChannelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "secretnamespace",
			},
		},
		want: func() string {
			fe := apis.ErrDisallowedFields("target.Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe.Error()
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name:       fromChannelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "",
			},
		},
		want: "",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidCallable(test.c).Error()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("isValidFrom (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidResultStrategy(t *testing.T) {
	targetURI := "http://example.com"
	tests := []struct {
		name string
		c    Callable
		want string
	}{{
		name: "valid target",
		c:    *getValidCall(),
		want: "",
	}, {
		name: "valid targetURI",
		c: Callable{
			TargetURI: &targetURI,
		},
		want: "",
	}, {
		name: "both target and targetURI given",
		c: Callable{
			Target: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
			TargetURI: &targetURI,
		},
		want: func() string {
			fe := apis.ErrMultipleOneOf("target", "targetURI")
			return fe.Error()
		}(),
	}, {
		name: "missing name in target",
		c: Callable{
			Target: &corev1.ObjectReference{
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("target.name")
			return fe.Error()
		}(),
	}, {
		name: "missing apiVersion in target",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name: fromChannelName,
				Kind: channelKind,
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("target.apiVersion")
			return fe.Error()
		}(),
	}, {
		name: "missing kind in target",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name:       fromChannelName,
				APIVersion: channelAPIVersion,
			},
		},
		want: func() string {
			fe := apis.ErrMissingField("target.kind")
			return fe.Error()
		}(),
	}, {
		name: "extra field, namespace",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name:       fromChannelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "secretnamespace",
			},
		},
		want: func() string {
			fe := apis.ErrDisallowedFields("target.Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe.Error()
		}(),
	}, {
		// Make sure that if an empty field for namespace is given, it's treated as not there.
		name: "valid extra field, namespace empty",
		c: Callable{
			Target: &corev1.ObjectReference{
				Name:       fromChannelName,
				APIVersion: channelAPIVersion,
				Kind:       channelKind,
				Namespace:  "",
			},
		},
		want: "",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isValidCallable(test.c).Error()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("isValidFrom (-want, +got) = %v", diff)
			}
		})
	}
}
