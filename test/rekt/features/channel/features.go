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

package channel

import (
	"fmt"

	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/containersource"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/source"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/resources/svc"
)

func ChannelChain(length int) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	cs := feature.MakeRandomK8sName("containersource")

	var channels []string
	for i := 0; i < length; i++ {
		name := feature.MakeRandomK8sName(fmt.Sprintf("channel-%04d", i))
		channels = append(channels, name)
		f.Setup("install channel", channel_impl.Install(name))
		f.Requirement("channel is ready", channel_impl.IsReady(name))
	}

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	// attach the first channel to the source
	f.Setup("install containersource", containersource.Install(cs, pingsource.WithSink(channel_impl.AsRef(channels[0]), "")))

	// use the rest for the chain
	for i := 0; i < length; i++ {
		sub := feature.MakeRandomK8sName(fmt.Sprintf("subscription-%04d", i))
		if i == length-1 {
			// install the final connection to the sink
			f.Setup("install sink subscription", subscription.Install(sub,
				subscription.WithChannel(channel_impl.AsRef(channels[i])),
				subscription.WithReply(svc.AsKReference(sink), ""),
			))
		} else {
			f.Setup("install subscription", subscription.Install(sub,
				subscription.WithChannel(channel_impl.AsRef(channels[i])),
				subscription.WithReply(channel_impl.AsRef(channels[i+1]), ""),
			))
		}
	}
	f.Requirement("containersource goes ready", containersource.IsReady(cs))

	f.Assert("chained channels relay events", assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}

func DeadLetterSink() *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	cs := feature.MakeRandomK8sName("containersource")
	name := feature.MakeRandomK8sName("channel")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install channel", channel_impl.Install(name, delivery.WithDeadLetterSink(svc.AsKReference(sink), "")))
	f.Setup("install containersource", containersource.Install(cs, source.WithSink(channel_impl.AsRef(name), "")))
	f.Setup("install subscription", subscription.Install(feature.MakeRandomK8sName("subscription"),
		subscription.WithChannel(channel_impl.AsRef(name)),
		subscription.WithReply(svc.AsKReference("does-not-exist"), ""),
	))

	f.Requirement("channel is ready", channel_impl.IsReady(name))
	f.Requirement("containersource is ready", containersource.IsReady(cs))

	f.Assert("dls receives events", assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}
