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

package retry_after

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/subscription"
)

const (
	ChannelNameKey       = "ChannelNameKey"
	SubscriptionNameKey  = "SubscriptionNameKey"
	SenderNameKey        = "SenderNameKey"
	ReceiverNameKey      = "ReceiverNameKey"
	RetryAttemptsKey     = "RetryAttemptsKey"
	RetryAfterSecondsKey = "RetryAfterSecondsKey"
)

// ConfigureDataPlane creates a Feature which sets up the specified Channel,
// Subscription and EventsHub Receiver so that it is ready to receive CloudEvents.
func ConfigureDataPlane(ctx context.Context, t *testing.T) *feature.Feature {

	// Get Component Names From Context
	var retryAttempts, retryAfterSeconds int
	channelName := state.GetStringOrFail(ctx, t, ChannelNameKey)
	subscriptionName := state.GetStringOrFail(ctx, t, SubscriptionNameKey)
	receiverName := state.GetStringOrFail(ctx, t, ReceiverNameKey)
	state.GetOrFail(ctx, t, RetryAttemptsKey, &retryAttempts)
	state.GetOrFail(ctx, t, RetryAfterSecondsKey, &retryAfterSeconds)

	// Create A Feature To Configure The DataPlane (Channel, Subscription, Receiver)
	f := feature.NewFeatureNamed("Configure Data-Plane")
	f.Setup("Install An EventsHub Receiver", eventshub.Install(receiverName,
		eventshub.StartReceiver,
		eventshub.DropFirstN(uint(retryAttempts)),
		eventshub.DropEventsResponseCode(http.StatusTooManyRequests),
		eventshub.DropEventsResponseHeaders(map[string]string{"Retry-After": strconv.Itoa(retryAfterSeconds)})))
	f.Setup("Install A Channel", channel.Install(channelName))
	f.Setup("Install A Subscription", installRetryAfterSubscription(channelName, subscriptionName, receiverName, int32(retryAttempts)))
	f.Assert("Channel Is Ready", channel.IsReady(channelName))
	f.Assert("Subscription Is Ready", subscription.IsReady(subscriptionName))

	// Return The ConfigureDataPlane Feature
	return f
}

// SendEvent creates a Feature which sends a CloudEvents to the specified
// Channel and verifies the timing of its receipt in the corresponding
// EventsHub Receiver. It is assumed that the backing Channel / Subscription
// / Receiver are in place and ready to receive the event.
func SendEvent(ctx context.Context, t *testing.T) *feature.Feature {

	// Get Component Names From Context
	var retryAttempts, retryAfterSeconds int
	channelName := state.GetStringOrFail(ctx, t, ChannelNameKey)
	senderName := state.GetStringOrFail(ctx, t, SenderNameKey)
	receiverName := state.GetStringOrFail(ctx, t, ReceiverNameKey)
	state.GetOrFail(ctx, t, RetryAttemptsKey, &retryAttempts)
	state.GetOrFail(ctx, t, RetryAfterSecondsKey, &retryAfterSeconds)

	// Create The Base CloudEvent To Send (ID will be set by the EventsHub Sender)
	event := cetest.FullEvent()

	// Mark The Current Time Plus The Expected Retry-After Duration
	earliestEventTime := time.Now().Add(time.Duration(retryAfterSeconds) * time.Second)

	// Create A New Feature To Send An Event And Verify Retry-After Duration
	f := feature.NewFeatureNamed("Send Events")
	f.Setup("Install An EventsHub Sender", eventshub.Install(senderName, eventshub.StartSenderToResource(channel.GVR(), channelName), eventshub.InputEvent(event)))
	f.Assert("Events Received", assert.OnStore(receiverName).MatchEvent(cetest.HasId(event.ID())).Exact(retryAttempts+1)) // One Successful Response
	f.Assert("Event Timing Verified", assert.OnStore(receiverName).Match(receivedAfterEventInfoMatcher(earliestEventTime)).Exact(retryAttempts))

	// Return The SendEvents Feature
	return f
}

// installRetryAfterSubscription performs the installation of a Subscription
// for the specified Channel / Sink in the test namespace.  The Subscription
// is configured to specify the experimental RetryAfter behavior which the
// standard Subscription resource/yaml does not include.
func installRetryAfterSubscription(channelName, subscriptionName, sinkName string, retryAttempts int32) func(ctx context.Context, t feature.T) {
	return func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()
		channelAPIVersion, imcKind := channel.GVK().ToAPIVersionAndKind()
		backoffPolicy := eventingduckv1.BackoffPolicyLinear
		retryAfterSubscription := &messagingv1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      subscriptionName,
				Namespace: namespace,
			},
			Spec: messagingv1.SubscriptionSpec{
				Channel: duckv1.KReference{
					APIVersion: channelAPIVersion,
					Kind:       imcKind,
					Name:       channelName,
				},
				Subscriber: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       sinkName,
					},
				},
				Delivery: &eventingduckv1.DeliverySpec{
					Retry:         pointer.Int32Ptr(retryAttempts),
					BackoffPolicy: &backoffPolicy,
					BackoffDelay:  pointer.StringPtr("PT0.5S"),
					RetryAfterMax: pointer.StringPtr("PT30S"),
				},
			},
		}
		_, err := eventingclient.Get(ctx).MessagingV1().Subscriptions(namespace).Create(ctx, retryAfterSubscription, metav1.CreateOptions{})
		require.NoError(t, err)
	}
}

// receivedAfterEventInfoMatcher returns an EventInfoMatcher that validates
// the receipt time of the EventInfo is AFTER the specified minimum time.
func receivedAfterEventInfoMatcher(earliestTime time.Time) eventshub.EventInfoMatcher {
	return func(eventInfo eventshub.EventInfo) error {
		if eventInfo.Time.Before(earliestTime) {
			return fmt.Errorf("response received prior to Retry-After header duration")
		}
		return nil
	}
}
