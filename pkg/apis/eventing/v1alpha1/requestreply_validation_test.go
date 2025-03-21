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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestRequestReplyValidation(t *testing.T) {
	tests := []struct {
		name string
		rr   *RequestReply
		want *apis.FieldError
	}{
		{
			name: "valid, all required fields set",
			rr: &RequestReply{
				Spec: RequestReplySpec{
					ReplyAttribute:       "reply",
					CorrelationAttribute: "correlate",
					Secrets:              []string{"secret1"},
					BrokerRef: duckv1.KReference{
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
						Name:       "broker",
					},
				},
			},
			want: func() *apis.FieldError {
				return nil
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := apis.WithinCreate(context.Background())
			got := test.rr.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate EventPolicySpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
