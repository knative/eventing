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

package eventtype_autocreate

import (
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func AutoCreateEventTypesOnIMC() *feature.Feature {
	f := feature.NewFeature()

	event := cetest.FullEvent()
	event.SetType("test.custom.event.type")

	sender := feature.MakeRandomK8sName("sender")
	sub := feature.MakeRandomK8sName("subscription")
	channelName := feature.MakeRandomK8sName("channel")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install channel", channel_impl.Install(channelName))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(channelName)),
		subscription.WithSubscriber(service.AsKReference(sink), ""),
	))

	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))

	f.Requirement("install event sender", eventshub.Install(sender,
		eventshub.StartSenderToResource(channel_impl.GVR(), channelName),
		eventshub.InputEvent(event),
	))

	expectedTypes := sets.New(event.Type())

	f.Alpha("imc").
		Must("deliver events to subscriber", assert.OnStore(sink).MatchEvent(cetest.HasId(event.ID())).AtLeast(1)).
		Must("create event type", eventtype.WaitForEventType(eventtype.AssertPresent(expectedTypes)))

	return f
}
