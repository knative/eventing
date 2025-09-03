/*
Copyright 2025 The Knative Authors

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

package requestreply

import (
	"fmt"
	"os"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	reconcilertesting "knative.dev/pkg/reconciler/testing"

	// Fake injection informer
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret/fake"
)

const (
	eventType   = "dev.knative.example"
	eventSource = "knativesource"
	eventID     = "1234567890"
)

func TestCESQLFilterWithVerifyCorrelationId(t *testing.T) {
	tests := map[string]struct {
		getEvent    func() *cloudevents.Event
		want        eventfilter.FilterResult
		replyIdName string
		expression  string
	}{
		"has matching replyid": {
			getEvent: func() *cloudevents.Event {
				ce := makeEvent()
				SetCorrelationId(ce, "replyid", exampleKey, 0)
				return ce
			},
			want: eventfilter.PassFilter,
		},
		"has matching replyid on second secret": {
			getEvent: func() *cloudevents.Event {
				ce := makeEvent()
				SetCorrelationId(ce, "replyid", otherKey, 0)
				return ce
			},
			want:       eventfilter.PassFilter,
			expression: "KN_VERIFY_CORRELATION_ID(replyid, \"request-reply\", \"default\", \"request-reply-keys-2\", 0, 1)",
		},
		"no matching secret for replyid": {
			getEvent: func() *cloudevents.Event {
				ce := makeEvent()
				SetCorrelationId(ce, "replyid", otherKey, 0)
				return ce
			},
			want: eventfilter.FailFilter,
		},
		"malformed replyid": {
			getEvent: func() *cloudevents.Event {
				ce := makeEvent()
				ce.SetExtension("replyid", fmt.Sprintf("%s:", eventID))
				return ce
			},
			want: eventfilter.FailFilter,
		},
		"no replyid": {
			getEvent: makeEvent,
			want:     eventfilter.FailFilter,
		},
	}

	ctx, _ := reconcilertesting.SetupFakeContext(t)

	_ = secretinformer.Get(ctx).Informer().GetStore().Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "request-reply-keys",
			Namespace: "knative-eventing",
		},
		Data: map[string][]byte{
			"default.request-reply.key": exampleKey,
		},
	})

	_ = secretinformer.Get(ctx).Informer().GetStore().Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "request-reply-keys-2",
			Namespace: "knative-eventing",
		},
		Data: map[string][]byte{
			"default.request-reply.key-1": exampleKey,
			"default.request-reply.key-2": otherKey,
		},
	})

	os.Setenv("SYSTEM_NAMESPACE", "knative-eventing")

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := RegisterCESQLVerifyCorrelationIdFilter(ctx)
			assert.NoError(t, err, "registering the correlation id function should not fail")

			if tc.replyIdName == "" {
				tc.replyIdName = "replyid"
			}

			if tc.expression == "" {
				tc.expression = fmt.Sprintf("KN_VERIFY_CORRELATION_ID(%s, \"request-reply\", \"default\", \"request-reply-keys\", 0, 1)", tc.replyIdName)
			}

			ce := tc.getEvent()

			filter, err := subscriptionsapi.NewCESQLFilter(tc.expression)
			assert.NoError(t, err, "parsing the CESQL filter should succeed")

			result := filter.Filter(ctx, *ce)
			assert.Equal(t, tc.want, result, "cesql result should match expected result")
		})
	}
}

func makeEvent() *cloudevents.Event {
	ce := cloudevents.NewEvent()
	ce.SetType(eventType)
	ce.SetSource(eventSource)
	ce.SetID(eventID)

	return &ce
}
