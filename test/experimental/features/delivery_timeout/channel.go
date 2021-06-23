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

package delivery_timeout

import (
	"context"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/rickb777/date/period"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/subscription"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
)

// ChannelToSink tests a scenario where the flow is source -> channel -> sink (timeout) -- fallback to -> dead letter sink
func ChannelToSink() *feature.Feature {
	f := feature.NewFeature()

	timeout := 6 * time.Second
	// Clocks are funny, let's add 1 second to take in account eventual clock skews
	timeoutPeriod, _ := period.NewOf(timeout - time.Second)
	timeoutString := timeoutPeriod.String()

	channelAPIVersion, imcKind := channel.GVK().ToAPIVersionAndKind()

	channelName := feature.MakeRandomK8sName("channel")
	subName := feature.MakeRandomK8sName("sub-sink")
	sinkName := feature.MakeRandomK8sName("sink")
	deadLetterSinkName := feature.MakeRandomK8sName("dead-letter")
	sourceName := feature.MakeRandomK8sName("source")

	ev := cetest.FullEvent()

	f.Setup("install sink", eventshub.Install(
		sinkName,
		eventshub.StartReceiver,
		eventshub.ResponseWaitTime(timeout),
	))
	f.Setup("install dead letter sink", eventshub.Install(
		deadLetterSinkName,
		eventshub.StartReceiver,
	))

	f.Setup("Install channel", channel.Install(channelName))
	f.Setup("Channel is ready", channel.IsReady(channelName))

	f.Setup("Install channel -> sink subscription", func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()
		_, err := eventingclient.Get(ctx).MessagingV1().Subscriptions(namespace).Create(ctx,
			&messagingv1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      subName,
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
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "Service",
								Name:       deadLetterSinkName,
							},
						},
						Timeout: &timeoutString,
					},
				},
			}, metav1.CreateOptions{})
		require.NoError(t, err)
	})

	f.Setup("subscription channel -> Sink is ready", subscription.IsReady(subName))

	f.Setup("install source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(channel.GVR(), channelName),
		eventshub.InputEvent(ev),
	))

	f.Assert("receive event on sink", assert.OnStore(sinkName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))
	f.Assert("receive event on dead letter sink", assert.OnStore(deadLetterSinkName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))

	return f
}
