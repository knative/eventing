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

package broker

import (
	"context"
	"errors"

	"github.com/google/uuid"

	. "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
	eventshubmain "knative.dev/reconciler-test/pkg/test_images/eventshub"
)

func DataPlaneConformance(brokerName string) *feature.FeatureSet {
	return &feature.FeatureSet{
		Name: "Knative Broker Specification - Data Plane",
		Features: []feature.Feature{
			*DataPlaneIngress(brokerName),
			*DataPlaneDelivery(brokerName),
			*DataPlaneObservability(brokerName),
		},
	}
}

func DataPlaneIngress(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Ingress")

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Conformance").
		Should("A Broker SHOULD expose either an HTTP or HTTPS endpoint as ingress. It MAY expose both.",
			todo).
		Must("The ingress endpoint(s) MUST conform to at least one of the following versions of the specification: 0.3, 1.0",
			brokerAcceptsCEVersions).
		May("Other versions MAY be rejected.",
			todo).
		ShouldNot("The Broker SHOULD NOT perform an upgrade of the produced event's CloudEvents version.",
			todo).
		Should("It SHOULD support both Binary Content Mode and Structured Content Mode of the HTTP Protocol Binding for CloudEvents.",
			todo).
		May("The HTTP(S) endpoint MAY be on any port, not just the standard 80 and 443.",
			todo).
		May("Channels MAY expose other, non-HTTP endpoints in addition to HTTP at their discretion.",
			todo).
		Must("Brokers MUST reject all HTTP produce requests with a method other than POST responding with HTTP status code `405 Method Not Supported`.",
			todo).
		Must("The Broker MUST respond with a 200-level HTTP status code if a produce request is accepted.",
			todo).
		Must("If a Broker receives a produce request and is unable to parse a valid CloudEvent, then it MUST reject the request with HTTP status code `400 Bad Request`.",
			todo)

	return f
}

func DataPlaneDelivery(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Delivery")

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Conformance").
		Must("Delivered events MUST conform to the CloudEvents specification.",
			todo).
		Should("All CloudEvent attributes set by the producer, including the data and specversion attributes, SHOULD be received at the subscriber identical to how they were received by the Broker.",
			todo).
		Should("The Broker SHOULD support delivering events via Binary Content Mode or Structured Content Mode of the HTTP Protocol Binding for CloudEvents.",
			todo).
		Should("Events accepted by the Broker SHOULD be delivered at least once to all subscribers of all Triggers*.",
			todo).
		//*
		//1. are Ready when the produce request was received,
		//1. specify filters that match the event, and
		//1. exist when the event is able to be delivered.
		May("Events MAY additionally be delivered to Triggers that become Ready after the event was accepted.",
			todo).
		May("Events MAY be enqueued or delayed between acceptance from a producer and delivery to a subscriber.",
			todo).
		May("The Broker MAY choose not to deliver an event due to persistent unavailability of a subscriber or limitations such as storage capacity.",
			todo).
		Should("The Broker SHOULD attempt to notify the operator in this case.",
			todo).
		May("The Broker MAY forward these events to an alternate endpoint or storage mechanism such as a dead letter queue.",
			todo).
		May("If no ready Trigger would match an accepted event, the Broker MAY drop that event without notifying the producer.",
			todo).
		May("If multiple Triggers reference the same subscriber, the subscriber MAY be expected to acknowledge successful delivery of an event multiple times.",
			todo).
		Should("Events contained in delivery responses SHOULD be published to the Broker ingress and processed as if the event had been produced to the Broker's addressable endpoint.",
			todo).
		Should("Events contained in delivery responses that are malformed SHOULD be treated as if the event delivery had failed.",
			todo).
		May("The subscriber MAY receive a confirmation that a reply event was accepted by the Broker.",
			todo).
		Should("If the reply event was not accepted, the initial event SHOULD be redelivered to the subscriber.",
			todo)

	return f
}

func DataPlaneObservability(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Observability")

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Conformance").
		Should("The Broker SHOULD expose a variety of metrics*.",
			todo).
		// * including, but not limited to:
		//- Number of malformed produce requests (400-level responses)
		//- Number of accepted produce requests (200-level responses)
		//- Number of events delivered

		Should("Metrics SHOULD be enabled by default, with a configuration parameter included to disable them if desired.",
			todo).
		Should("Upon receiving an event with context attributes defined in the CloudEvents Distributed Tracing extension the Broker SHOULD preserve that trace header on delivery to subscribers and on reply events, unless the reply is sent with a different set of tracing attributes.",
			todo).
		Should("Forwarded trace headers SHOULD be updated with any intermediate spans emitted by the broker.",
			todo).
		Should("Spans emitted by the Broker SHOULD follow the OpenTelemetry Semantic Conventions for Messaging System*.",
			todo)
	// *
	//- messaging.system: "knative"
	//- messaging.destination: broker:name.namespace or trigger:name.namespace with
	//the Broker or Trigger to which the event is being routed
	//- messaging.protocol: the name of the underlying transport protocol
	//- messaging.message_id: the event ID

	return f
}

func todo(ctx context.Context, t feature.T) {
	t.Log("TODO, Implement this.")
}

func brokerAcceptsCEVersions(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	u, err := broker.Address(ctx, brokerName)
	if err != nil || u == nil {
		t.Error("failed to get the address of the broker", brokerName, err)
	}

	opts := []eventshub.EventsHubOption{eventshub.StartSenderURL(u.String())}
	uuids := map[string]string{
		uuid.New().String(): "1.0",
		uuid.New().String(): "0.3",
	}
	for uuid, version := range uuids {
		// We need to use a different source name, otherwise, it will try to update
		// the pod, which is immutable.
		source := feature.MakeRandomK8sName("source")
		event := FullEvent()
		event.SetID(uuid)
		event.SetSpecVersion(version)
		opts = append(opts, eventshub.InputEvent(event))

		eventshub.Install(source, opts...)(ctx, t)
		store := eventshub.StoreFromContext(ctx, source)
		// We are looking for two events, one of them is the sent event and the other
		// is Response. We want to make sure the event was sent and that it was accepted.
		// TODO: Figure out how to send events via eventshub that
		events := store.AssertAtLeast(2, sentEventMatcher(uuid))
		for _, e := range events {
			if e.Kind == eventshubmain.EventResponse {
				// Make sure HTTP response code is 2XX
				if e.StatusCode < 200 || e.StatusCode > 299 {
					t.Errorf("Expected statuscode 2XX got %d", e.StatusCode)
				}
			}
		}
	}
}

func sentEventMatcher(uuid string) func(eventshubmain.EventInfo) error {
	return func(ei eventshubmain.EventInfo) error {
		if ei.Kind == eventshubmain.EventSent && ei.Event.ID() == uuid {
			return nil
		}
		if ei.Kind == eventshubmain.EventResponse {
			return nil
		}
		return errors.New("no match")
	}
}
