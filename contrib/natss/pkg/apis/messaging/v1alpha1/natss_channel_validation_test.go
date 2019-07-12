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
	"knative.dev/pkg/webhook"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
)

func TestNatssChannelValidation(t *testing.T) {
	testCases := map[string]struct {
		cr   webhook.GenericCRD
		want *apis.FieldError
	}{
		"empty spec": {
			cr: &NatssChannel{
				Spec: NatssChannelSpec{},
			},
			want: nil,
		},
		"valid subscribers array": {
			cr: &NatssChannel{
				Spec: NatssChannelSpec{
					Subscribable: &eventingduck.Subscribable{
						Subscribers: []eventingduck.SubscriberSpec{{
							SubscriberURI: "subscriberendpoint",
							ReplyURI:      "resultendpoint",
						}},
					}},
			},
			want: nil,
		},
		"empty subscriber at index 1": {
			cr: &NatssChannel{
				Spec: NatssChannelSpec{
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
		},
		"two empty subscribers": {
			cr: &NatssChannel{
				Spec: NatssChannelSpec{
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
		},
	}

	for n, test := range testCases {
		t.Run(n, func(t *testing.T) {
			got := test.cr.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validate (-want, +got) = %v", n, diff)
			}
		})
	}
}
