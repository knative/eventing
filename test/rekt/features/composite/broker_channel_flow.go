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

package composite

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

// BrokerChannelFlow topology:
//
//
//                                                       2
//                         +------------------------------------------------------+
//                         |                                                      |
//                         |                                                      |
//                         |          +----------+                          +-----+-----+
//                         |     1    |          |          1               |           |
//                         |  +------>| trigger1 +------------------------->| transform1|
//                         |  |       |          |                          |           |
//                         |  |       +----------+                          +-----------+
//                         v  |
// +------------+     +-------+--+    +----------+                          +-----------+
// |            | 1   |          | 2  |          |          2               |           |
// |  Source    +---->|  Broker  +----> trigger2 +------------------------->| logger    |
// |            |     |          |    |          |                  3       |           |
// +------------+     +-------+--+    +----------+            +------------>+-----------+
//                            |                               |
//                            |       +----------+      +-----+------+      +-----------+        +-----------+
//                            |  2    |          |  2   |            |  2   |           |        |           |
//                            +------>| trigger3 +----->| channel    +----->| subscript +------->| transform2|
//                                    |          |      |            |      |           |        |           |
//                                    +----------+      +------------+      +-----------+        +-----+-----+
//                                                            ^                                        |
//                                                            |                  3                     |
//                                                            +----------------------------------------+
//
func BrokerChannelFlow() *feature.Feature {

	f := feature.NewFeatureNamed("broker channel flow")

	sourceName := feature.MakeRandomK8sName("source")
	brokerName := feature.MakeRandomK8sName("broker")
	trigger1 := feature.MakeRandomK8sName("trigger-1")
	trigger2 := feature.MakeRandomK8sName("trigger-2")
	trigger3 := feature.MakeRandomK8sName("trigger-3")
	channelName := feature.MakeRandomK8sName("channel")
	subscriptionName := feature.MakeRandomK8sName("subscription")

	prober := eventshub.NewProber()
	prober.SetTargetResource(broker.GVR(), brokerName)

	sourceEvent := cloudevents.NewEvent()
	sourceEvent.SetID(uuid.New().String())
	sourceEvent.SetSource("EventSource")
	sourceEvent.SetType("tobetransformed")
	_ = sourceEvent.SetData("text/plain", "source-data")

	transformedEvent1 := cloudevents.NewEvent()
	transformedEvent1.SetID(uuid.New().String())
	transformedEvent1.SetSource("EventSource")
	transformedEvent1.SetType("transformed")
	_ = transformedEvent1.SetData("text/plain", "transformed-data-trigger")

	transformedEvent2 := cloudevents.NewEvent()
	transformedEvent2.SetID(uuid.New().String())
	transformedEvent2.SetSource("EventSource")
	transformedEvent2.SetType("transformed-subscription")
	_ = transformedEvent2.SetData("text/plain", "transformed-data-subscription")

	f.Setup("install transformation 1", prober.ReceiverInstall(trigger1,
		eventshub.ReplyWithTransformedEvent(transformedEvent1.Type(), transformedEvent1.Source(), string(transformedEvent1.Data())),
	))
	f.Setup("install transformation 2", prober.ReceiverInstall(subscriptionName,
		eventshub.ReplyWithTransformedEvent(transformedEvent2.Type(), transformedEvent2.Source(), string(transformedEvent2.Data())),
	))
	f.Setup("install logger", prober.ReceiverInstall(trigger2))

	f.Setup("install channel", channel_impl.Install(channelName))
	f.Setup("install subscription", subscription.Install(subscriptionName,
		subscription.WithChannel(channel_impl.AsRef(channelName)),
		subscription.WithSubscriber(prober.AsKReference(subscriptionName), ""),
		subscription.WithReply(prober.AsKReference(trigger2), ""),
	))

	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("install trigger 1", trigger.Install(trigger1, brokerName,
		trigger.WithSubscriber(prober.AsKReference(trigger1), ""),
		trigger.WithFilter(map[string]string{"type": sourceEvent.Type()}),
	))
	f.Setup("install trigger 2", trigger.Install(trigger2, brokerName,
		trigger.WithSubscriber(prober.AsKReference(trigger2), ""),
		trigger.WithFilter(map[string]string{"type": transformedEvent1.Type()}),
	))
	f.Setup("install trigger 3", trigger.Install(trigger3, brokerName,
		trigger.WithSubscriber(prober.AsKReference(subscriptionName), ""),
		trigger.WithFilter(map[string]string{"type": transformedEvent1.Type()}),
	))

	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("trigger 1 is ready", trigger.IsReady(trigger1))
	f.Setup("trigger 2 is ready", trigger.IsReady(trigger2))
	f.Setup("trigger 3 is ready", trigger.IsReady(trigger3))
	f.Setup("subscription is ready", subscription.IsReady(subscriptionName))

	f.Requirement("install event source", prober.SenderInstall(sourceName,
		eventshub.AddTracing,
		eventshub.InputEvent(sourceEvent),
	))
	f.Requirement("sender done", prober.SenderDone(sourceName))

	f.Assert("event source finished", prober.AssertSentAll(sourceName))

	f.Assert("transformation service received events from source", prober.AssertReceivedAll(sourceName, trigger1))

	// Flow 1, trigger1 -> transformation1
	f.Assert("trigger 1 received event from source",
		eventasssert.OnStore(prober.AsKReference(trigger1).Name).Match(
			eventasssert.MatchKind(eventasssert.EventReceived),
			eventasssert.MatchEvent(
				cetest.HasType(sourceEvent.Type()),
				cetest.DataContains(string(sourceEvent.Data())),
				cetest.HasSource(sourceEvent.Source()),
			),
		).AtLeast(1),
	)

	// Flow 2, trigger2 -> logger
	f.Assert("trigger 2 received event from transformation 1 pod",
		eventasssert.OnStore(prober.AsKReference(trigger2).Name).Match(
			eventasssert.MatchKind(eventasssert.EventReceived),
			eventasssert.MatchEvent(
				cetest.HasType(transformedEvent1.Type()),
				cetest.DataContains(string(transformedEvent1.Data())),
				cetest.HasSource(transformedEvent1.Source()),
			),
		).AtLeast(1),
	)
	// Flow 2, trigger3 -> subscription
	f.Assert("subscription received event from transformation 1 pod",
		eventasssert.OnStore(prober.AsKReference(subscriptionName).Name).Match(
			eventasssert.MatchKind(eventasssert.EventReceived),
			eventasssert.MatchEvent(
				cetest.HasType(transformedEvent1.Type()),
				cetest.DataContains(string(transformedEvent1.Data())),
				cetest.HasSource(transformedEvent1.Source()),
			),
		).AtLeast(1),
	)

	// Flow 3, subscription -> logger
	f.Assert("trigger 2 received event from transformation 2 pod",
		eventasssert.OnStore(prober.AsKReference(trigger2).Name).Match(
			eventasssert.MatchKind(eventasssert.EventReceived),
			eventasssert.MatchEvent(
				cetest.HasType(transformedEvent2.Type()),
				cetest.DataContains(string(transformedEvent2.Data())),
				cetest.HasSource(transformedEvent2.Source()),
			),
		).AtLeast(1),
	)

	return f
}
