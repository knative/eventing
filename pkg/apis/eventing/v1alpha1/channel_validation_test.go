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
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var dnsName = "example.com"

func TestChannelValidation(t *testing.T) {
	tests := []CRDTest{{
		name: "valid",
		cr: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		want: nil,
	}, {
		name: "empty",
		cr: &Channel{
			Spec: ChannelSpec{},
		},
		want: apis.ErrMissingField("spec.provisioner"),
	}, {
		name: "subscribers array",
		cr: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.ChannelSubscriberSpec{{
						SubscriberURI: "subscriberendpoint",
						ReplyURI:      "resultendpoint",
					}},
				}},
		},
		want: nil,
	}, {
		name: "empty subscriber at index 1",
		cr: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.ChannelSubscriberSpec{{
						SubscriberURI: "subscriberendpoint",
						ReplyURI:      "replyendpoint",
					}, {}},
				}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
			fe.Details = "expected at least one of, got none"
			return fe
		}(),
	}, {
		name: "2 empty subscribers",
		cr: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.ChannelSubscriberSpec{{}, {}},
				},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("spec.subscribable.subscriber[0].replyURI", "spec.subscribable.subscriber[0].subscriberURI")
			fe.Details = "expected at least one of, got none"
			errs = errs.Also(fe)
			fe = apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
			fe.Details = "expected at least one of, got none"
			errs = errs.Also(fe)
			return errs
		}(),
	}}

	doValidateTest(t, tests)
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
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		old:  nil,
		want: nil,
	}, {
		name: "good (no change)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		want: nil,
	}, {
		name: "good (arguments change)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
				Arguments: &runtime.RawExtension{
					Raw: []byte("\"foo\":\"bar\""),
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
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
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
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
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "bar",
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
