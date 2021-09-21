/*
Copyright 2021 The Knative Authors

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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestDeliverySpecSetDefaults(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name  string
		given *DeliverySpec
		want  *DeliverySpec
		ctx   context.Context
	}{
		{
			name: "nil",
			ctx:  context.Background(),
		},
		{
			name:  "deadLetterSink nil",
			ctx:   context.Background(),
			given: &DeliverySpec{},
			want:  &DeliverySpec{},
		},
		{
			name:  "deadLetterSink.ref nil",
			ctx:   context.Background(),
			given: &DeliverySpec{DeadLetterSink: &duckv1.Destination{}},
			want:  &DeliverySpec{DeadLetterSink: &duckv1.Destination{}},
		},
		{
			name: "deadLetterSink.ref.namespace empty string",
			ctx:  apis.WithinParent(context.Background(), metav1.ObjectMeta{Name: "b", Namespace: "custom"}),
			given: &DeliverySpec{DeadLetterSink: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Kind:       "Service",
					Namespace:  "",
					Name:       "svc",
					APIVersion: "v1",
				},
			}},
			want: &DeliverySpec{DeadLetterSink: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Kind:       "Service",
					Namespace:  "custom",
					Name:       "svc",
					APIVersion: "v1",
				},
			}},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.given.SetDefaults(tc.ctx)
			if diff := cmp.Diff(tc.want, tc.given); diff != "" {
				t.Error("(-want, +got)", diff)
			}
		})
	}
}
