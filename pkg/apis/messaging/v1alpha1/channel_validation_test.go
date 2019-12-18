/*
Copyright 2019 The Knative Authors

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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
)

func TestChannelValidation(t *testing.T) {
	tests := []CRDTest{{
		name: "empty",
		cr: &Channel{
			Spec: ChannelSpec{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.channelTemplate")
			return fe
		}(),
	}, {
		name: "channel template with no kind",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						APIVersion: SchemeGroupVersion.String(),
					},
				}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.channelTemplate.kind")
			return fe
		}(),
	}, {
		name: "channel template with no apiVersion",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind: "InMemoryChannel",
					},
				}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.channelTemplate.apiVersion")
			return fe
		}(),
	}, {
		name: "valid subscribers array",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("resultendpoint"),
					}},
				}},
		},
		want: nil,
	}, {
		name: "empty subscriber at index 1",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}, {}},
				}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
			fe.Details = "expected at least one of, got none"
			return fe
		}(),
	}, {
		name: "nil channelTemplate and empty subscriber at index 1",
		cr: &Channel{
			Spec: ChannelSpec{
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}, {}},
				}},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("spec.channelTemplate")
			errs = errs.Also(fe)
			fe = apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
			fe.Details = "expected at least one of, got none"
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "2 empty subscribers",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{}, {}},
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
		name     string
		current  *Channel
		original *Channel
		want     *apis.FieldError
	}{{
		name: "good (no change)",
		current: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
					Spec: &runtime.RawExtension{
						Raw: []byte(`"foo":"baz"`),
					},
				},
			},
		},
		original: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
					Spec: &runtime.RawExtension{
						Raw: []byte(`"foo":"baz"`),
					},
				},
			},
		},
		want: nil,
	}, {
		name: "new nil is ok",
		current: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
					Spec: &runtime.RawExtension{
						Raw: []byte(`"foo":"baz"`),
					},
				},
			},
		},
		original: nil,
		want:     nil,
	}, {
		name: "bad (channelTemplate change)",
		current: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "OtherChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
					Spec: &runtime.RawExtension{
						Raw: []byte(`"foo":"baz"`),
					},
				},
			},
		},
		original: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
					Spec: &runtime.RawExtension{
						Raw: []byte(`"foo":"baz"`),
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.ChannelSpec}.ChannelTemplate.TypeMeta.Kind:
	-: "InMemoryChannel"
	+: "OtherChannel"
`,
		},
	}, {
		name: "good (subscribable change)",
		current: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}},
				},
			},
		},
		original: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
				},
			},
		},
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.current.CheckImmutableFields(context.TODO(), test.original)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}
