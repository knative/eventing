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
	"testing"

	"golang.org/x/net/context"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
)

var (
	validIMCSingleSubscriber = &InMemoryChannel{
		Spec: InMemoryChannelSpec{
			ChannelableSpec: eventingduck.ChannelableSpec{
				SubscribableSpec: eventingduck.SubscribableSpec{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("resultendpoint"),
					}},
				}},
		},
	}

	validIMCTwoSubscribers = &InMemoryChannel{
		Spec: InMemoryChannelSpec{
			ChannelableSpec: eventingduck.ChannelableSpec{
				SubscribableSpec: eventingduck.SubscribableSpec{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("resultendpoint"),
					}, {
						SubscriberURI: apis.HTTP("subscriberendpoint2"),
						ReplyURI:      apis.HTTP("resultendpoint2"),
					}},
				}},
		},
	}
)

func TestInMemoryChannelValidation(t *testing.T) {
	tests := []CRDTest{{
		name: "empty",
		cr: &InMemoryChannel{
			Spec: InMemoryChannelSpec{},
		},
		want: nil,
	}, {
		name: "valid subscribers array",
		cr: &InMemoryChannel{
			Spec: InMemoryChannelSpec{
				ChannelableSpec: eventingduck.ChannelableSpec{
					SubscribableSpec: eventingduck.SubscribableSpec{
						Subscribers: []eventingduck.SubscriberSpec{{
							SubscriberURI: apis.HTTP("subscriberendpoint"),
							ReplyURI:      apis.HTTP("resultendpoint"),
						}},
					}},
			},
		},
		want: nil,
	}, {
		name: "empty subscriber at index 1",
		cr: &InMemoryChannel{
			Spec: InMemoryChannelSpec{
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
			fe := apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
			fe.Details = "expected at least one of, got none"
			return fe
		}(),
	}, {
		name: "2 empty subscribers",
		cr: &InMemoryChannel{
			Spec: InMemoryChannelSpec{
				ChannelableSpec: eventingduck.ChannelableSpec{
					SubscribableSpec: eventingduck.SubscribableSpec{
						Subscribers: []eventingduck.SubscriberSpec{{}, {}},
					},
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
	}, {
		name: "invalid scope annotation",
		cr: &InMemoryChannel{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					eventing.ScopeAnnotationKey: "notvalid",
				},
			},
			Spec: InMemoryChannelSpec{},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrInvalidValue("notvalid", "metadata.annotations.[eventing.knative.dev/scope]")
			fe.Details = "expected either 'cluster' or 'namespace'"
			return fe
		}(),
	}, {
		name: "invalid user for spec.subscribers update",
		cr:   validIMCTwoSubscribers,
		want: func() *apis.FieldError {
			diff, _ := kmp.ShortDiff(validIMCSingleSubscriber.Spec.Subscribers, validIMCTwoSubscribers.Spec.Subscribers)
			return &apis.FieldError{
				Message: "Channel.Spec.Subscribers changed by user test-user which was not the system:serviceaccount:knative-eventing:eventing-controller service account",
				Paths:   []string{"spec.subscribers"},
				Details: diff,
			}
		}(),
		ctx: apis.WithUserInfo(apis.WithinUpdate(context.TODO(), validIMCSingleSubscriber), &authenticationv1.UserInfo{Username: "test-user"}),
	}}

	doValidateTest(t, tests)
}
