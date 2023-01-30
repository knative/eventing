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

package kreference_group

import (
	"context"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"

	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/subscription"
)

// SubscriptionWithChannelGroup tests a scenario where the flow is source -> channel -> sink
// where the channel -> sink subscription uses the KReference.Group field for Subscription.Spec.Channel
func SubscriptionWithChannelGroup() *feature.Feature {
	f := feature.NewFeature()

	channelGVK := channel.GVK()
	channelGVR := channel.GVR()
	channelGroup := channelGVK.GroupKind().Group
	_, channelKind := channelGVK.ToAPIVersionAndKind()

	channelName := feature.MakeRandomK8sName("channel")
	subName := feature.MakeRandomK8sName("sub")
	sinkName := feature.MakeRandomK8sName("sink")
	sourceName := feature.MakeRandomK8sName("source")

	ev := cetest.FullEvent()

	f.Setup("install sink", eventshub.Install(
		sinkName,
		eventshub.StartReceiver,
	))

	f.Setup("Install channel", channel.Install(channelName))
	f.Setup("channel is ready", channel.IsReady(channelName))

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
						Group: channelGroup,
						Kind:  channelKind,
						Name:  channelName,
					},
					Subscriber: &duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "Service",
							Name:       sinkName,
						},
					},
				},
			}, metav1.CreateOptions{})
		require.NoError(t, err)
	})

	f.Setup("subscription channel -> sink is ready", subscription.IsReady(subName))

	f.Requirement("install source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(channelGVR, channelName),
		eventshub.InputEvent(ev),
	))

	f.Assert("receive event", assert.OnStore(sinkName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))

	return f
}
