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
	"fmt"

	"github.com/google/uuid"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/flaker"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"

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
	cfg := []manifest.CfgFn{trigger.WithSubscriber(svc.AsRef(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))

	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Setup("install source", func(ctx context.Context, t feature.T) {
		u, err := broker.Address(ctx, brokerName)
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

// SourceToSinkWithDLQ tests to see if a Ready Broker acts as middleware.
//
// source ---> broker --[trigger]--> bad uri
//                  |
//                  +--[DLQ]--> recorder
//
func SourceToSinkWithDLQ(brokerName string) *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	dlq := feature.MakeRandomK8sName("dlq")
	via := feature.MakeRandomK8sName("via")
	event := FullEvent()

	f := new(feature.Feature)

	f.Setup("install sink", svc.Install(sink, "bad", "svc"))

	f.Setup("install dlq", eventshub.Install(dlq, eventshub.StartReceiver))

	f.Setup("update broker with DLQ", broker.Install(brokerName, broker.WithDeadLetterSink(svc.AsRef(dlq), "")))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(svc.AsRef(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))

	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Setup("install source", func(ctx context.Context, t feature.T) {
		u, err := broker.Address(ctx, brokerName)
		if err != nil || u == nil {
			t.Error("failed to get the address of the broker", brokerName, err)
		}
		eventshub.Install(source, eventshub.StartSenderURL(u.String()), eventshub.InputEvent(event))(ctx, t)
	})

	f.Stable("broker with DQL").
		Must("deliver event to DLQ",
			OnStore(dlq).MatchEvent(HasId(event.ID())).Exact(1))

	return f
}

// SourceToSinkWithFlakyDLQ tests to see if a Ready Broker acts as middleware.
//
// source ---> broker --[trigger]--> flake 1/3 --> recorder
//                  |
//                  +--[DLQ]--> recorder
//
func SourceToSinkWithFlakyDLQ(brokerName string) *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	flake := feature.MakeRandomK8sName("flake")
	dlq := feature.MakeRandomK8sName("dlq")
	via := feature.MakeRandomK8sName("via")

	uuids := []string{
		uuid.New().String(),
		uuid.New().String(),
		uuid.New().String(),
	}

	f := new(feature.Feature)

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Setup("install flake", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		u := fmt.Sprintf("%s.%s.svc.cluster.local", sink, env.Namespace()) // HACK HACK HACK, could replace with SinkBinding.
		flaker.Install(flake, u)(ctx, t)
	})

	f.Setup("install dlq", eventshub.Install(dlq, eventshub.StartReceiver))

	f.Setup("update broker with DLQ", broker.Install(brokerName, broker.WithDeadLetterSink(svc.AsRef(dlq), "")))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(flaker.AsRef(flake), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))

	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Setup("install source", func(ctx context.Context, t feature.T) {
		u, err := broker.Address(ctx, brokerName)
		if err != nil || u == nil {
			t.Error("failed to get the address of the broker", brokerName, err)
		}

		opts := []eventshub.EventsHubOption{eventshub.StartSenderURL(u.String())}
		for _, id := range uuids {
			event := FullEvent()
			event.SetID(id)
			opts = append(opts, eventshub.InputEvent(event))
		}
		eventshub.Install(source, opts...)(ctx, t)
	})

	f.Stable("broker with DQL").
		Must("deliver event flaky sent to DLQ event[0]",
			OnStore(dlq).MatchEvent(HasId(uuids[0])).Exact(1)).
		Must("deliver event flaky sent to DLQ event[1]",
			OnStore(dlq).MatchEvent(HasId(uuids[1])).Exact(1)).
		Must("deliver event sink receiver got event[2]",
			OnStore(sink).MatchEvent(HasId(uuids[2])).Exact(1))

	return f
}
