/*
Copyright 2020 The Knative Authors

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

package broker

import (
	"context"
	"testing"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/eventing/test/rekt/features"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/eventing/test/rekt/resources/trigger"

	. "github.com/cloudevents/sdk-go/v2/test"
	. "knative.dev/reconciler-test/pkg/eventshub/assert"
)

// SourceToSink tests to see if a Ready Broker acts as middleware.
// LoadGenerator --> in [Broker] out --> Recorder
func SourceToSink(brokerName string) *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")
	event := FullEvent()

	f := new(feature.Feature)

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []trigger.CfgFn{trigger.WithSubscriber(svc.AsRef(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))

	f.Setup("trigger goes ready", trigger.IsReady(via, features.Interval, features.Timeout))

	f.Setup("install source", func(ctx context.Context, t *testing.T) {
		u, err := broker.Address(ctx, brokerName, features.Interval, features.Timeout)
		if err != nil || u == nil {
			t.Error("failed to get the address of the broker", brokerName, err)
		}
		eventshub.Install(source, eventshub.StartSenderURL(u.String()), eventshub.InputEvent(event))(ctx, t)
	})

	f.Stable("broker as middleware").
		Must("deliver an event",
			OnStore(sink).MatchEvent(HasId(event.ID())).Exact(1))

	return f
}
