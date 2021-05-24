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

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
)

// ChannelToChannel tests a scenario where the flow is source -> A -> B -> sink
// and A -> B subscription uses the KReference.Group field.
func ChannelToChannel() *feature.Feature {
	f := feature.NewFeature()

	channelAName := feature.MakeRandomK8sName("channel-a")
	channelBName := feature.MakeRandomK8sName("channel-b")
	subAToBName := feature.MakeRandomK8sName("sub-a-b")
	subBToSinkName := feature.MakeRandomK8sName("sub-b-sink")
	sinkName := feature.MakeRandomK8sName("sink")
	sourceName := feature.MakeRandomK8sName("source")

	f.Setup("install sink", eventshub.Install(
		sinkName,
		eventshub.StartReceiver,
	))

	f.Setup("Install channel A", installInMemoryChannel(channelAName))
	f.Setup("Install channel B", installInMemoryChannel(channelBName))
	f.Setup("Install channel B -> sink subscription", func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()
		_, err := eventingclient.Get(ctx).MessagingV1().Subscriptions(namespace).Create(ctx,
			&messagingv1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name: subAToBName,
					Namespace: namespace,
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel:
				},
			}, metav1.CreateOptions{})
		require.NoError(t, err)
	})
}

func installInMemoryChannel(channelName string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()
		_, err := eventingclient.Get(ctx).MessagingV1().InMemoryChannels(namespace).Create(ctx,
			&messagingv1.InMemoryChannel{
				ObjectMeta: metav1.ObjectMeta{
					Name: channelName,
					Namespace: namespace,
				},
			}, metav1.CreateOptions{})
		require.NoError(t, err)
	}
}