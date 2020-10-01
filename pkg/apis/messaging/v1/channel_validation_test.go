/*
Copyright 2020 The Knative Authors

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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
)

func TestChannelValidation(t *testing.T) {
	tests := []CRDTest{{
		name: "empty",
		cr: &Channel{
			Spec: ChannelSpec{},
		},
		want: apis.ErrMissingField("spec.channelTemplate"),
	}, {
		name: "channel template with no kind",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						APIVersion: SchemeGroupVersion.String(),
					},
				}},
		},
		want: apis.ErrMissingField("spec.channelTemplate.kind"),
	}, {
		name: "channel template with no apiVersion",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind: "InMemoryChannel",
					},
				}},
		},
		want: apis.ErrMissingField("spec.channelTemplate.apiVersion"),
	}, {
		name: "invalid subscribers array, not allowed",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelTemplate: &ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
				},
				ChannelableSpec: eventingduck.ChannelableSpec{
					SubscribableSpec: eventingduck.SubscribableSpec{
						Subscribers: []eventingduck.SubscriberSpec{{
							SubscriberURI: apis.HTTP("subscriberendpoint"),
							ReplyURI:      apis.HTTP("resultendpoint"),
						}},
					}},
			}},
		want: apis.ErrDisallowedFields("spec.subscribable.subscribers"),
	}, {
		name: "nil channelTemplate and disallowed at index 1",
		cr: &Channel{
			Spec: ChannelSpec{
				ChannelableSpec: eventingduck.ChannelableSpec{
					SubscribableSpec: eventingduck.SubscribableSpec{
						Subscribers: []eventingduck.SubscriberSpec{{
							SubscriberURI: apis.HTTP("subscriberendpoint"),
							ReplyURI:      apis.HTTP("replyendpoint"),
						}, {}},
					}},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			errs = errs.Also(apis.ErrMissingField("spec.channelTemplate"))
			errs = errs.Also(apis.ErrDisallowedFields("spec.subscribable.subscribers"))
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
				ChannelTemplate: &ChannelTemplateSpec{
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
				ChannelTemplate: &ChannelTemplateSpec{
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
				ChannelTemplate: &ChannelTemplateSpec{
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
				ChannelTemplate: &ChannelTemplateSpec{
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
				ChannelTemplate: &ChannelTemplateSpec{
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
			Details: `{v1.ChannelSpec}.ChannelTemplate.TypeMeta.Kind:
	-: "InMemoryChannel"
	+: "OtherChannel"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = apis.WithinUpdate(ctx, test.original)
			got := test.current.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("CheckImmutableFields (-want, +got) =", diff)
			}
		})
	}
}
