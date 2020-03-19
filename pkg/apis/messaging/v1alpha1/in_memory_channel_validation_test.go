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
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/testutils"
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
		cr: &InMemoryChannel{
			Spec: InMemoryChannelSpec{
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
		name: "2 empty subscribers",
		cr: &InMemoryChannel{
			Spec: InMemoryChannelSpec{
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
	}}

	doValidateTest(t, tests)
}

func TestChannel_CheckImmutableFields(t *testing.T) {

	type tc struct {
		name      string
		ctx       context.Context
		imcs      *InMemoryChannel
		wantError bool
	}

	imcp := func(annotations map[string]string) *InMemoryChannel {
		return &InMemoryChannel{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: annotations,
			},
		}
	}

	transitions := testutils.GetScopeAnnotationsTransitions()
	tt := make([]tc, len(transitions))

	for i, t := range transitions {
		tt[i] = tc{
			name:      fmt.Sprintf("original %s current %s", t.Original[eventing.ScopeAnnotationKey], t.Current[eventing.ScopeAnnotationKey]),
			ctx:       apis.WithinUpdate(context.TODO(), imcp(t.Original)),
			imcs:      imcp(t.Current),
			wantError: t.WantError,
		}
	}

	tt = append(tt, tc{
		name:      "no original",
		ctx:       context.TODO(),
		imcs:      &InMemoryChannel{},
		wantError: false,
	})

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.imcs.checkImmutableFields(tc.ctx)
			if tc.wantError != (errs != nil) {
				t.Fatalf("want error %v got %+v", tc.wantError, errs)
			}
		})
	}
}
