package requestreply

import (
	"fmt"
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
				SetCorrelationId(ce, "replyid", exampleKey)
				return ce
			},
			want: eventfilter.PassFilter,
		},
		"has matching replyid on second secret": {
			getEvent: func() *cloudevents.Event {
				ce := makeEvent()
				SetCorrelationId(ce, "replyid", otherKey)
				return ce
			},
			want:       eventfilter.PassFilter,
			expression: "KN_VERIFY_CORRELATION_ID(replyid, \"default\", \"request-reply-secret-1\", \"request-reply-secret-2\")",
		},
		"no matching secret for replyid": {
			getEvent: func() *cloudevents.Event {
				ce := makeEvent()
				SetCorrelationId(ce, "replyid", otherKey)
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
			Name:      "request-reply-secret-1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"key": exampleKey,
		},
	})

	_ = secretinformer.Get(ctx).Informer().GetStore().Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "request-reply-secret-2",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"key": otherKey,
		},
	})

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := RegisterCESQLVerifyCorrelationIdFilter(ctx)
			assert.NoError(t, err, "registering the correlation id function should not fail")

			if tc.replyIdName == "" {
				tc.replyIdName = "replyid"
			}

			if tc.expression == "" {
				tc.expression = fmt.Sprintf("KN_VERIFY_CORRELATION_ID(%s, \"default\", \"request-reply-secret-1\")", tc.replyIdName)
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
