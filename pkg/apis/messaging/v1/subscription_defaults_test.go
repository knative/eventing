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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func TestSubscriptionDefaults(t *testing.T) {
	s := Subscription{}
	s.SetDefaults(context.TODO())

	tt := []struct {
		name  string
		given *Subscription
		want  *Subscription
	}{
		{
			name: "subscription empty",
		},
		{
			name:  "subscription.spec nil",
			given: &Subscription{},
			want:  &Subscription{},
		},
		{
			name: "subscription.spec empty",
			given: &Subscription{
				Spec: SubscriptionSpec{},
			},
			want: &Subscription{
				Spec: SubscriptionSpec{},
			},
		},
		{
			name: "subscription.spec.subscriber empty",
			given: &Subscription{
				Spec: SubscriptionSpec{
					Subscriber: &duckv1.Destination{},
				},
			},
			want: &Subscription{
				Spec: SubscriptionSpec{
					Subscriber: &duckv1.Destination{},
				},
			},
		},
		{
			name: "subscription.spec.reply empty",
			given: &Subscription{
				Spec: SubscriptionSpec{
					Reply: &duckv1.Destination{},
				},
			},
			want: &Subscription{
				Spec: SubscriptionSpec{
					Reply: &duckv1.Destination{},
				},
			},
		},
		{
			name: "subscription.spec.delivery empty",
			given: &Subscription{
				Spec: SubscriptionSpec{
					Delivery: &eventingduckv1.DeliverySpec{},
				},
			},
			want: &Subscription{
				Spec: SubscriptionSpec{
					Delivery: &eventingduckv1.DeliverySpec{},
				},
			},
		},
		{
			name: "subscription.spec.subscriber.ref.namespace empty",
			given: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "custom",
					Name:      "s",
				},
				Spec: SubscriptionSpec{
					Subscriber: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "Service",
							Name:       "svc",
							APIVersion: "v1",
						},
					},
				},
			},
			want: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "custom",
					Name:      "s",
				},
				Spec: SubscriptionSpec{
					Subscriber: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "Service",
							Name:       "svc",
							APIVersion: "v1",
							Namespace:  "custom",
						},
					},
				},
			},
		},
		{
			name: "subscription.spec.reply.ref.namespace empty",
			given: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "custom",
					Name:      "s",
				},
				Spec: SubscriptionSpec{
					Reply: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "Service",
							Name:       "svc",
							APIVersion: "v1",
						},
					},
				},
			},
			want: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "custom",
					Name:      "s",
				},
				Spec: SubscriptionSpec{
					Reply: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "Service",
							Name:       "svc",
							APIVersion: "v1",
							Namespace:  "custom",
						},
					},
				},
			},
		},
		{
			name: "subscription.spec.delivery.deadLetterSink.ref.namespace empty",
			given: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "custom",
					Name:      "s",
				},
				Spec: SubscriptionSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Name:       "svc",
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "custom",
					Name:      "s",
				},
				Spec: SubscriptionSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "custom",
								Name:       "svc",
								APIVersion: "v1",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.given.SetDefaults(context.Background())
			if diff := cmp.Diff(tc.want, tc.given); diff != "" {
				t.Error("(-want, +got)", diff)
			}
		})
	}
}
