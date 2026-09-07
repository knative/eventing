/*
Copyright 2026 The Knative Authors

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

package backoff_max

import (
	"context"
	"net/http"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/subscription"
)

// ChannelToSink verifies that BackoffMax caps retries in the data plane.
func ChannelToSink() *feature.Feature {
	f := feature.NewFeatureNamed("Delivery backoff maximum")

	channelName := feature.MakeRandomK8sName("backoff-max-channel")
	subscriptionName := feature.MakeRandomK8sName("backoff-max-subscription")
	receiverName := feature.MakeRandomK8sName("backoff-max-receiver")
	senderName := feature.MakeRandomK8sName("backoff-max-sender")
	event := cetest.FullEvent()

	f.Setup("install receiver", eventshub.Install(
		receiverName,
		eventshub.StartReceiver,
		eventshub.DropFirstN(4),
		eventshub.DropEventsResponseCode(http.StatusServiceUnavailable),
	))
	f.Setup("install channel", channel.Install(channelName))
	f.Setup("install subscription", installSubscription(channelName, subscriptionName, receiverName))
	f.Requirement("channel is ready", channel.IsReady(channelName))
	f.Requirement("subscription is ready", subscription.IsReady(subscriptionName))
	f.Assert("send event", eventshub.Install(
		senderName,
		eventshub.StartSenderToResource(channel.GVR(), channelName),
		eventshub.InputEvent(event),
	))

	assertBackoffMax(f, receiverName, event.ID())

	return f
}

func installSubscription(channelName, subscriptionName, sinkName string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()
		channelAPIVersion, channelKind := channel.GVK().ToAPIVersionAndKind()
		backoffPolicy := eventingduckv1.BackoffPolicyExponential
		retry := int32(4)
		_, err := eventingclient.Get(ctx).MessagingV1().Subscriptions(namespace).Create(ctx, &messagingv1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      subscriptionName,
				Namespace: namespace,
			},
			Spec: messagingv1.SubscriptionSpec{
				Channel: duckv1.KReference{
					APIVersion: channelAPIVersion,
					Kind:       channelKind,
					Name:       channelName,
				},
				Subscriber: &duckv1.Destination{Ref: &duckv1.KReference{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       sinkName,
				}},
				Delivery: &eventingduckv1.DeliverySpec{
					Retry:         &retry,
					BackoffPolicy: &backoffPolicy,
					BackoffDelay:  pointer.String("PT1S"),
					BackoffMax:    pointer.String("PT2S"),
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	}
}
