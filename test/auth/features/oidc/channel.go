/*
Copyright 2023 The Knative Authors

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

package oidc

import (
	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func ChannelDispatcherAuthenticatesRequestsWithOIDC() *feature.Feature {
	f := feature.NewFeatureNamed("Channel dispatcher authenticates requests with OIDC")

	source := feature.MakeRandomK8sName("source")
	channelName := feature.MakeRandomK8sName("channel")
	sink := feature.MakeRandomK8sName("sink")
	subscriptionName := feature.MakeRandomK8sName("subscription")
	receiverAudience := feature.MakeRandomK8sName("receiver")

	f.Setup("install channel", channel_impl.Install(channelName))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("install sink", eventshub.Install(sink, eventshub.OIDCReceiverAudience(receiverAudience), eventshub.StartReceiver))
	f.Setup("install subscription", subscription.Install(subscriptionName, subscription.WithChannel(channel_impl.AsRef(channelName)), subscription.WithSubscriber(service.AsKReference(sink), "", receiverAudience)))
	f.Setup("subscription is ready", subscription.IsReady(subscriptionName))

	event := test.FullEvent()
	f.Requirement("install source", eventshub.Install(source, eventshub.InputEvent(event), eventshub.StartSenderToResource(channel_impl.GVR(), channelName)))

	f.Alpha("channel dispatcher").Must("authenticate requests with OIDC", assert.OnStore(sink).MatchReceivedEvent(test.HasId(event.ID())).AtLeast(1))

	return f
}
