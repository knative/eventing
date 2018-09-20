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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var targetURI = "https://example.com"

func TestChannelValidation(t *testing.T) {
	tests := []struct {
		name string
		c    *Channel
		want *apis.FieldError
	}{{
		name: "valid",
		c: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
			},
		},
		want: nil,
	}, {
		name: "empty",
		c: &Channel{
			Spec: ChannelSpec{},
		},
		want: apis.ErrMissingField("spec.provisioner"),
	}, {
		name: "subscribers array",
		c: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Subscribers: []ChannelSubscriberSpec{{
					Call: &Callable{
						TargetURI: &targetURI,
					},
				}, {
					Result: &ResultStrategy{
						Target: &corev1.ObjectReference{
							APIVersion: "eventing.knative.dev/v1alpha1",
							Kind:       "Channel",
							Name:       "to-chan",
						},
					},
				}, {
					Call: &Callable{
						TargetURI: &targetURI,
					},
					Result: &ResultStrategy{
						Target: &corev1.ObjectReference{
							APIVersion: "eventing.knative.dev/v1alpha1",
							Kind:       "Channel",
							Name:       "to-chan",
						},
					},
				}},
			},
		},
		want: nil,
	}, {
		name: "empty subscriber",
		c: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Subscribers: []ChannelSubscriberSpec{{
					Call: &Callable{
						TargetURI: &targetURI,
					},
				}, {}},
			},
		},
		want: apis.ErrMissingField("spec.subscriber[1].call", "spec.subscriber[1].result"),
	}, {
		name: "2 empty subscribers",
		c: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Subscribers: []ChannelSubscriberSpec{{}, {}},
			},
		},
		want: apis.ErrMissingField("spec.subscriber[0].call", "spec.subscriber[0].result").
			Also(apis.ErrMissingField("spec.subscriber[1].call", "spec.subscriber[1].result")),
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

func TestChannelImmutableFields(t *testing.T) {
	tests := []struct {
		name string
		new  apis.Immutable
		old  apis.Immutable
		want *apis.FieldError
	}{{
		name: "good (new)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
			},
		},
		old:  nil,
		want: nil,
	}, {
		name: "good (no change)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
			},
		},
		want: nil,
	}, {
		name: "good (arguments change)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Arguments: &runtime.RawExtension{
					Raw: []byte("\"foo\":\"bar\""),
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"foo":"baz"}`),
				},
			},
		},
		want: nil,
	}, {
		name: "bad (not channel)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
			},
		},
		old: &Subscription{},
		want: &apis.FieldError{
			Message: "The provided resource was not a Channel",
		},
	}, {
		name: "bad (provisioner changes)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &ProvisionerReference{
					Ref: &corev1.ObjectReference{
						Name: "bar",
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed",
			Paths:   []string{"spec.provisioner"},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.new.CheckImmutableFields(test.old)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("Validate (-want, +got) = %v", diff)
			}
		})
	}
}
