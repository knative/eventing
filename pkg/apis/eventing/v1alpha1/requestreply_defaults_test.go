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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/google/go-cmp/cmp"
)

func TestRequestReplyDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  RequestReply
		expected RequestReply
	}{
		"nil spec": {
			initial: RequestReply{},
			expected: RequestReply{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"eventing.knative.dev/broker": "",
					},
				},
				Spec: RequestReplySpec{
					Timeout:              ptr.To("PT30S"),
					CorrelationAttribute: "correlationid",
					ReplyAttribute:       "replyid",
				},
			},
		},
		"does not override existing values": {
			initial: RequestReply{
				Spec: RequestReplySpec{
					Timeout:              ptr.To("PT40S"),
					CorrelationAttribute: "othercorrelationid",
					ReplyAttribute:       "otherreplyid",
				},
			},
			expected: RequestReply{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"eventing.knative.dev/broker": "",
					},
				},
				Spec: RequestReplySpec{
					Timeout:              ptr.To("PT40S"),
					CorrelationAttribute: "othercorrelationid",
					ReplyAttribute:       "otherreplyid",
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.TODO())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatal("Unexpected defaults (-want, +got):", diff)
			}
		})
	}
}
