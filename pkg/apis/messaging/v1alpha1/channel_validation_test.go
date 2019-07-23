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
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
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
				ChannelTemplate: &eventingduck.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind:       "InMemoryChannel",
						APIVersion: SchemeGroupVersion.String(),
					},
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
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
		name: "nil channelTemplate and empty subscriber at index 1",
		cr: &Channel{
			Spec: ChannelSpec{
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: "subscriberendpoint",
						ReplyURI:      "replyendpoint",
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
