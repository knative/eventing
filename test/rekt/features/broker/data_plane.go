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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	test "github.com/cloudevents/sdk-go/v2/test"

	"github.com/google/uuid"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/rekt/features/knconf"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/pkg/apis"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
	"knative.dev/reconciler-test/pkg/state"
)

func DataPlaneConformance(brokerName string) *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Broker Specification - Data Plane",
		Features: []*feature.Feature{
			DataPlaneIngress(brokerName),
			DataPlaneObservability(brokerName),
			DataPlaneAddressability(brokerName),
		},
	}

	addDataPlaneDelivery(brokerName, fs)

	return fs
}

func DataPlaneIngress(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Ingress")

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Conformance").
		Must("The ingress endpoint(s) MUST conform to at least one of the following versions of the specification: 0.3, 1.0",
			brokerAcceptsCEVersions).
		May("Other versions MAY be rejected.",
			brokerRejectsUnknownCEVersion).
		ShouldNot("The Broker SHOULD NOT perform an upgrade of the produced event's CloudEvents version.",
			brokerEventVersionNotUpgraded).
		Should("It SHOULD support Binary Content Mode of the HTTP Protocol Binding for CloudEvents.",
			brokerAcceptsBinaryContentMode).
		Should("It SHOULD support Structured Content Mode of the HTTP Protocol Binding for CloudEvents.",
			brokerAcceptsStructuredContentMode).
		Must("Brokers MUST reject all HTTP produce requests with a method other than POST responding with HTTP status code `405 Method Not Supported`.",
			brokerRejectsGetRequest).
		Must("The Broker MUST respond with a 200-level HTTP status code if a produce request is accepted.",
			brokerAcceptResponseSuccess).
		Must("If a Broker receives a produce request and is unable to parse a valid CloudEvent, then it MUST reject the request with HTTP status code `400 Bad Request`.",
			brokerRejectsMalformedCE)
	return f
}

func DataPlaneAddressability(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Broker Addressability")

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Requirement("Broker is Ready", broker.IsAddressable(brokerName))

	f.Stable("Conformance").Should("A Broker SHOULD expose either an HTTP or HTTPS endpoint as ingress. It MAY expose both.",
		func(ctx context.Context, t feature.T) {
			b := getBroker(ctx, t)
			addr := b.Status.AddressStatus.Address.URL
			if addr == nil {
				addr = new(apis.URL)
			}
			if addr.Scheme != "http" && addr.Scheme != "https" {
				t.Fatalf("expected broker scheme to be HTTP or HTTPS, found: %s", addr.Scheme)
			}
		}).
		May("The HTTP(S) endpoint MAY be on any port, not just the standard 80 and 443.",
			icebox).
		May("Channels MAY expose other, non-HTTP endpoints in addition to HTTP at their discretion.",
			icebox)
	return f
}

func addDataPlaneDelivery(brokerName string, fs *feature.FeatureSet) {
	f := feature.NewFeatureNamed("Delivery")

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Conformance").
		Must("Delivered events MUST conform to the CloudEvents specification.",
			deliveredCESpecs).
		Should("All CloudEvent attributes set by the producer, including the data and specversion attributes, SHOULD be received at the subscriber identical to how they were received by the Broker.",
			recievedCEAttributes).
		Should("The Broker SHOULD support delivering events via Binary Content Mode or Structured Content Mode of the HTTP Protocol Binding for CloudEvents.",
			brokerAcceptsBinaryandStructuredContentMode).
		Should("Events accepted by the Broker SHOULD be delivered at least once to all subscribers of all Triggers*.",
			allSubscribersRecieveCE).
		//*
		//1. are Ready when the produce request was received,
		//1. specify filters that match the event, and
		//1. exist when the event is able to be delivered.
		May("Events MAY additionally be delivered to Triggers that become Ready after the event was accepted.",
			triggerBecameReadyLater).
		May("Events MAY be enqueued or delayed between acceptance from a producer and delivery to a subscriber.",
			eventEnqueuedLater).
		May("The Broker MAY choose not to deliver an event due to persistent unavailability of a subscriber or limitations such as storage capacity.",
			icebox).
		Should("The Broker SHOULD attempt to notify the operator in this case.",
			icebox).
		May("The Broker MAY forward these events to an alternate endpoint or storage mechanism such as a dead letter queue.",
			icebox).
		May("If no ready Trigger would match an accepted event, the Broker MAY drop that event without notifying the producer.",
			noTriggersAvailible).
		May("If multiple Triggers reference the same subscriber, the subscriber MAY be expected to acknowledge successful delivery of an event multiple times.",
			multipleTriggerSameSink).
		Should("Events contained in delivery responses SHOULD be published to the Broker ingress and processed as if the event had been produced to the Broker's addressable endpoint.",
			replyEventDeliveredToSource).
		May("The subscriber MAY receive a confirmation that a reply event was accepted by the Broker.",
			todo).
		Should("If the reply event was not accepted, the initial event SHOULD be redelivered to the subscriber.",
			replyEventNotAccepted)

	fs.Features = append(fs.Features, f)

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

func todo(_ context.Context, t feature.T) {
	t.Log("TODO, Implement this.")
}

func icebox(_ context.Context, t feature.T) {
	t.Skip("[IceBox], It is not clear how to make a conformance test for this spec assertion.")
}

func brokerAcceptsCEVersions(ctx context.Context, t feature.T) {
	name := state.GetStringOrFail(ctx, t, "brokerName")
	knconf.AcceptsCEVersions(ctx, t, broker.GVR(), name)
}

func brokerAcceptsBinaryContentMode(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	contenttypes := []string{
		"application/vnd.apache.thrift.binary",
		"application/xml",
		"application/json",
	}
	for _, contenttype := range contenttypes {
		source := feature.MakeRandomK8sName("source")
		eventshub.Install(source,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputHeader("ce-specversion", "1.0"),
			eventshub.InputHeader("ce-type", "sometype"),
			eventshub.InputHeader("ce-source", "200.request.sender.test.knative.dev"),
			eventshub.InputHeader("ce-id", uuid.New().String()),
			eventshub.InputHeader("content-type", contenttype),
			eventshub.InputBody("{}"),
			eventshub.InputMethod("POST"),
		)(ctx, t)

		store := eventshub.StoreFromContext(ctx, source)
		events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
		for _, e := range events {
			if e.Response.StatusCode < 200 || e.Response.StatusCode > 299 {
				t.Errorf("Expected statuscode 2XX for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
			}
		}
	}
}

func brokerAcceptsStructuredContentMode(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	contenttype := "application/cloudevents+json"
	bodycontent := `{
    "specversion" : "1.0",
    "type" : "sometype",
    "source" : "json.request.sender.test.knative.dev",
    "id" : "2222-4444-6666",
    "time" : "2020-07-06T09:23:12Z",
    "datacontenttype" : "application/json",
    "data" : {
        "message" : "helloworld"
    }
}`
	source := feature.MakeRandomK8sName("source")
	eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputHeader("content-type", contenttype),
		eventshub.InputBody(bodycontent),
		eventshub.InputMethod("POST"),
	)(ctx, t)

	store := eventshub.StoreFromContext(ctx, source)
	events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
	for _, e := range events {
		if e.Response.StatusCode < 200 || e.Response.StatusCode > 299 {
			t.Errorf("Expected statuscode 2XX for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
		}
	}
}

func brokerRejectsUnknownCEVersion(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	uuids := map[string]string{
		uuid.New().String(): "19.0",
	}
	for id, version := range uuids {
		// We need to use a different source name, otherwise, it will try to update
		// the pod, which is immutable.
		source := feature.MakeRandomK8sName("source")
		eventshub.Install(source,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputHeader("ce-specversion", version),
			eventshub.InputHeader("ce-type", "sometype"),
			eventshub.InputHeader("ce-source", "400.request.sender.test.knative.dev"),
			eventshub.InputHeader("ce-id", id),
			eventshub.InputBody("{}"),
		)(ctx, t)

		store := eventshub.StoreFromContext(ctx, source)
		// We are looking for two events, one of them is the sent event and the other
		// is Response, so correlate them first. We want to make sure the event was sent and that the
		// response was what was expected.
		// Note: We pass in "" for the match ID because when we construct the headers manually
		// above, they do not get stuff into the sent/response SentId fields.
		events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
		for _, e := range events {
			// Make sure HTTP response code is 4XX
			if e.Response.StatusCode < 400 || e.Response.StatusCode > 499 {
				t.Errorf("Expected statuscode 4XX for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
			}
		}
	}
}

func brokerAcceptResponseSuccess(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	uuids := map[string]string{
		uuid.New().String(): "1.0",
	}
	for id, version := range uuids {
		source := feature.MakeRandomK8sName("source")
		eventshub.Install(source,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputHeader("ce-specversion", version),
			eventshub.InputHeader("ce-type", "sometype"),
			eventshub.InputHeader("ce-source", "200.request.sender.test.knative.dev"),
			eventshub.InputHeader("ce-id", id),
			eventshub.InputBody("{}"),
			eventshub.InputMethod("POST"),
		)(ctx, t)

		store := eventshub.StoreFromContext(ctx, source)
		events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
		for _, e := range events {
			// Make sure HTTP response code is 200
			if e.Response.StatusCode < 200 || e.Response.StatusCode > 299 {
				t.Errorf("Expected statuscode 200 for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
			}
		}
	}
}

func brokerRejectsGetRequest(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	uuids := map[string]string{
		uuid.New().String(): "1.0",
	}
	for id, version := range uuids {
		// We need to use a different source name, otherwise, it will try to update
		// the pod, which is immutable.
		source := feature.MakeRandomK8sName("source")
		eventshub.Install(source,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputHeader("ce-specversion", version),
			eventshub.InputHeader("ce-type", "sometype"),
			eventshub.InputHeader("ce-source", "400.request.sender.test.knative.dev"),
			eventshub.InputHeader("ce-id", id),
			eventshub.InputBody("{}"),
			eventshub.InputMethod("GET"),
		)(ctx, t)

		store := eventshub.StoreFromContext(ctx, source)
		// We are looking for two events, one of them is the sent event and the other
		// is Response, so correlate them first. We want to make sure the event was sent and that the
		// response was what was expected.
		// Note: We pass in "" for the match ID because when we construct the headers manually
		// above, they do not get stuff into the sent/response SentId fields.
		events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
		for _, e := range events {
			// Make sure HTTP response code is 405
			if e.Response.StatusCode != 405 {
				t.Errorf("Expected statuscode 405 for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
			}
		}
	}
}

func brokerRejectsMalformedCE(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")
	headers := map[string]string{
		"ce-specversion": "1.0",
		"ce-type":        "sometype",
		"ce-source":      "conformancetest.request.sender.test.knative.dev",
		"ce-id":          uuid.New().String(),
	}

	for k := range headers {
		// Add all but the one key we want to omit.
		// https://github.com/knative/eventing/issues/5143
		if k == "ce-type" || k == "ce-source" || k == "ce-id" {
			t.Logf("SKIPPING missing header %q due to known bug: https://github.com/knative/eventing/issues/5143", k)
			continue
		}

		var options []eventshub.EventsHubOption
		for k2, v2 := range headers {
			if k != k2 {
				options = append(options, eventshub.InputHeader(k2, v2))
				t.Logf("Adding Header Value: %q => %q", k2, v2)
			}
		}
		options = append(options, eventshub.StartSenderToResource(broker.GVR(), brokerName))
		options = append(options, eventshub.InputBody("{}"))
		// We need to use a different source name, otherwise, it will try to update
		// the pod, which is immutable.
		source := feature.MakeRandomK8sName("source")
		eventshub.Install(source, options...)(ctx, t)

		store := eventshub.StoreFromContext(ctx, source)
		// We are looking for two events, one of them is the sent event and the other
		// is Response, so correlate them first. We want to make sure the event was sent and that the
		// response was what was expected.
		// Note: We pass in "" for the match ID because when we construct the headers manually
		// above, they do not get stuff into the sent/response SentId fields.
		events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
		for _, e := range events {
			// Make sure HTTP response code is 4XX
			if e.Response.StatusCode < 400 || e.Response.StatusCode > 499 {
				t.Errorf("Expected statuscode 4XX with missing required field %q for sequence %d got %d", k, e.Response.Sequence, e.Response.StatusCode)
				t.Logf("Sent event was: %s\nresponse: %s\n", e.Sent.String(), e.Response.String())
			}
		}
	}
}

// source ---> [broker] ---[trigger]--> recorder
func brokerEventVersionNotUpgraded(ctx context.Context, t feature.T) {
	// brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	// Create a trigger,
}

func deliveredCESpecs(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	uuids := map[string]string{
		uuid.New().String(): "1.0",
		uuid.New().String(): "0.3",
	}

	for id, version := range uuids {
		source := feature.MakeRandomK8sName("source")
		sink := feature.MakeRandomK8sName("sink")
		triggerName := feature.MakeRandomK8sName("trigger")
		event := test.FullEvent()
		event.SetID(id)
		event.SetSpecVersion(version)

		// Creating sink
		eventshub.Install(sink, eventshub.StartReceiver)(ctx, t)

		// Point the Trigger subscriber to the sink svc.
		cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), "")}

		// Creating trigger
		trigger.Install(triggerName, brokerName, cfg...)

		eventshub.Install(source,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputEvent(event),
		)

		assert.OnStore(sink).MatchEvent(test.HasId(event.ID()), test.HasSpecVersion(version)).Exact(1)
	}
}

func recievedCEAttributes(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	event := test.FullEvent()

	// Creating sink
	eventshub.Install(sink, eventshub.StartReceiver)(ctx, t)

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), "")}

	// Creating trigger
	trigger.Install(triggerName, brokerName, cfg...)

	eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	)

	assert.OnStore(sink).MatchEvent(test.HasId(event.ID()), test.HasData(event.Data()), test.HasExactlyAttributesEqualTo(event.Context)).Exact(1)
}

func brokerAcceptsBinaryandStructuredContentMode(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	contenttypes := []string{
		"application/vnd.apache.thrift.binary",
		"application/xml",
		"application/json",
	}
	for _, contenttype := range contenttypes {
		source := feature.MakeRandomK8sName("source")
		eventshub.Install(source,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputHeader("ce-specversion", "1.0"),
			eventshub.InputHeader("ce-type", "sometype"),
			eventshub.InputHeader("ce-source", "200.request.sender.test.knative.dev"),
			eventshub.InputHeader("ce-id", uuid.New().String()),
			eventshub.InputHeader("content-type", contenttype),
			eventshub.InputBody("{}"),
			eventshub.InputMethod("POST"),
		)(ctx, t)

		store := eventshub.StoreFromContext(ctx, source)
		events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
		for _, e := range events {
			if e.Response.StatusCode < 200 || e.Response.StatusCode > 299 {
				t.Errorf("Expected statuscode 2XX for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
			}
		}
	}

	contenttype := "application/cloudevents+json"
	bodycontent := `{
    "specversion" : "1.0",
    "type" : "sometype",
    "source" : "json.request.sender.test.knative.dev",
    "id" : "2222-4444-6666",
    "time" : "2020-07-06T09:23:12Z",
    "datacontenttype" : "application/json",
    "data" : {
        "message" : "helloworld"
    }
	}`

	source := feature.MakeRandomK8sName("source")
	eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputHeader("content-type", contenttype),
		eventshub.InputBody(bodycontent),
		eventshub.InputMethod("POST"),
	)(ctx, t)

	store := eventshub.StoreFromContext(ctx, source)
	events := knconf.Correlate(store.AssertAtLeast(t, 2, knconf.SentEventMatcher("")))
	for _, e := range events {
		if e.Response.StatusCode < 200 || e.Response.StatusCode > 299 {
			t.Errorf("Expected statuscode 2XX for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
		}
	}
}

func allSubscribersRecieveCE(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")

	sink1 := feature.MakeRandomK8sName("sink1")
	sink2 := feature.MakeRandomK8sName("sink2")
	sink3 := feature.MakeRandomK8sName("sink3")
	sink4 := feature.MakeRandomK8sName("sink4")

	triggerName1 := feature.MakeRandomK8sName("trigger1")
	triggerName2 := feature.MakeRandomK8sName("trigger2")
	triggerName3 := feature.MakeRandomK8sName("trigger3")
	triggerName4 := feature.MakeRandomK8sName("trigger4")

	// Creating event
	event := test.FullEvent()

	// Creating sink
	eventshub.Install(sink1, eventshub.StartReceiver)(ctx, t)
	eventshub.Install(sink2, eventshub.StartReceiver)(ctx, t)
	eventshub.Install(sink3, eventshub.StartReceiver)(ctx, t)
	eventshub.Install(sink4, eventshub.StartReceiver)(ctx, t)

	// Creating trigger
	trigger.Install(triggerName1, brokerName, trigger.WithSubscriber(service.AsKReference(sink1), ""))
	trigger.Install(triggerName2, brokerName, trigger.WithSubscriber(service.AsKReference(sink2), ""))
	trigger.Install(triggerName3, brokerName, trigger.WithSubscriber(service.AsKReference(sink3), ""))
	trigger.Install(triggerName4, brokerName, trigger.WithSubscriber(service.AsKReference(sink4), ""))

	eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	)

	assert.OnStore(sink1).MatchEvent(test.HasId(event.ID())).Exact(1)
	assert.OnStore(sink2).MatchEvent(test.HasId(event.ID())).Exact(1)
	assert.OnStore(sink3).MatchEvent(test.HasId(event.ID())).Exact(1)
	assert.OnStore(sink4).MatchEvent(test.HasId(event.ID())).Exact(1)
}

func multipleTriggerSameSink(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	triggerName1 := feature.MakeRandomK8sName("trigger1")
	triggerName2 := feature.MakeRandomK8sName("trigger2")
	event := test.FullEvent()

	// Creating sink
	eventshub.Install(sink, eventshub.StartReceiver)(ctx, t)

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), "")}

	// Creating trigger
	trigger.Install(triggerName1, brokerName, cfg...)
	trigger.Install(triggerName2, brokerName, cfg...)

	eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	)

	// Since there are two triggers, there should be exactly two events
	assert.OnStore(sink).MatchEvent(test.HasId(event.ID())).Exact(2)
}

func triggerBecameReadyLater(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	event := test.FullEvent()

	f := new(feature.Feature)

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(triggerName, brokerName, cfg...))

	f.Setup("install source", eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Requirement("trigger goes ready", trigger.IsReady(triggerName))

	f.Stable("broker as middleware").
		Must("deliver an event",
			assert.OnStore(sink).MatchEvent(test.HasId(event.ID())).Exact(1))
}

func eventEnqueuedLater(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	event1 := test.FullEvent()
	event2 := test.FullEvent()

	f := new(feature.Feature)

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(triggerName, brokerName, cfg...))

	f.Requirement("trigger goes ready", trigger.IsReady(triggerName))

	f.Setup("install source", eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event1),
	))

	f.Setup("added another event between acceptance from a producer and delivery to a subscriber", eventshub.Install(source,
		eventshub.InputEvent(event2),
	))

	f.Stable("broker as middleware").
		Must("deliver an event",
			assert.OnStore(sink).MatchEvent(test.HasId(event1.ID())).Exact(1))

	f.Stable("broker as middleware").
		May("deliver an event",
			assert.OnStore(sink).MatchEvent(test.HasId(event2.ID())).Exact(1))
}

func noTriggersAvailible(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	event := test.FullEvent()

	// Creating sink
	eventshub.Install(sink, eventshub.StartReceiver)(ctx, t)

	eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	)

	// Since there are no triggers, there should be no events
	assert.OnStore(sink).Exact(0)

	// TODO: Check if source is notified or not
}

func replyEventDeliveredToSource(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")
	sink1 := feature.MakeRandomK8sName("sink1")
	sink2 := feature.MakeRandomK8sName("sink2")

	trigger1 := feature.MakeRandomK8sName("trigger1")
	trigger2 := feature.MakeRandomK8sName("trigger2")

	// Construct original cloudevent message
	eventType := "type1"
	eventSource := "http://source1.com"
	eventBody := `{"msg":"e2e-brokerchannel-body"}`

	// Construct reply event
	replyEventType := "type2"
	replyEventSource := "http://source2.com"
	replyBody := `{"msg":"reply body"}`

	// Construct eventToSend
	eventToSend := cloudevents.NewEvent()
	eventToSend.SetID(uuid.New().String())
	eventToSend.SetType(eventType)
	eventToSend.SetSource(eventSource)
	eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	eventshub.Install(sink1,
		eventshub.ReplyWithTransformedEvent(replyEventType, replyEventSource, replyBody),
		eventshub.StartReceiver)

	eventshub.Install(sink2, eventshub.StartReceiver)

	// filter1 filters the original events
	filter1 := eventingv1.TriggerFilterAttributes{
		"type":   eventType,
		"source": eventSource,
	}
	// filter2 filters events after transformation
	filter2 := eventingv1.TriggerFilterAttributes{
		"type":   replyEventType,
		"source": replyEventSource,
	}

	trigger.Install(
		trigger1,
		brokerName,
		trigger.WithFilter(filter1),
		trigger.WithSubscriber(service.AsKReference(sink1), ""),
	)

	trigger.Install(
		trigger2,
		brokerName,
		trigger.WithFilter(filter2),
		trigger.WithSubscriber(service.AsKReference(sink2), ""),
	)

	eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(eventToSend),
	)

	eventMatcher := eventasssert.MatchEvent(
		test.HasSource(eventSource),
		test.HasType(eventType),
		test.HasData([]byte(eventBody)),
	)
	transformEventMatcher := eventasssert.MatchEvent(
		test.HasSource(replyEventSource),
		test.HasType(replyEventType),
		test.HasData([]byte(replyBody)),
	)

	eventasssert.OnStore(sink2).Match(transformEventMatcher).AtLeast(1)
	eventasssert.OnStore(sink1).Match(eventMatcher).AtLeast(1)
	eventasssert.OnStore(sink2).Match(eventMatcher).Not()
}

func replyEventNotAccepted(ctx context.Context, t feature.T) {
	brokerName := state.GetStringOrFail(ctx, t, "brokerName")

	source := feature.MakeRandomK8sName("source")
	sink1 := feature.MakeRandomK8sName("sink1")
	sink2 := feature.MakeRandomK8sName("sink2")

	trigger1 := feature.MakeRandomK8sName("trigger1")
	trigger2 := feature.MakeRandomK8sName("trigger2")

	// Construct original cloudevent message
	eventType := "type1"
	eventSource := "http://source1.com"
	eventBody := `{"msg":"e2e-brokerchannel-body"}`

	// Construct reply event
	malformedReplyEventType := ""
	malformedReplyEventSource := ""
	malformedReplyBody := ``

	// Construct eventToSend
	eventToSend := cloudevents.NewEvent()
	eventToSend.SetID(uuid.New().String())
	eventToSend.SetType(eventType)
	eventToSend.SetSource(eventSource)
	eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	eventshub.Install(sink1,
		eventshub.ReplyWithTransformedEvent(malformedReplyEventType, malformedReplyEventSource, malformedReplyBody),
		eventshub.StartReceiver)

	eventshub.Install(sink2, eventshub.StartReceiver)

	// filter1 filters the original events
	filter1 := eventingv1.TriggerFilterAttributes{
		"type":   eventType,
		"source": eventSource,
	}

	trigger.Install(
		trigger1,
		brokerName,
		trigger.WithFilter(filter1),
		trigger.WithSubscriber(service.AsKReference(sink1), ""),
	)

	trigger.Install(
		trigger2,
		brokerName,
		trigger.WithSubscriber(service.AsKReference(sink2), ""),
	)

	eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(eventToSend),
	)

	transformEventMatcher := eventasssert.MatchEvent(
		test.HasSource(malformedReplyEventSource),
		test.HasType(malformedReplyEventType),
		test.HasData([]byte(malformedReplyBody)),
		test.IsInvalid(),
	)

	eventMatcher := eventasssert.MatchEvent(
		test.HasSource(eventSource),
		test.HasType(eventType),
		test.HasData([]byte(eventBody)),
		test.IsInvalid(),
	)

	eventasssert.OnStore(sink2).Match(transformEventMatcher).AtLeast(1)

	// Since the reply event was invalid, there should be atleast 2 events
	eventasssert.OnStore(sink1).Match(eventMatcher).AtLeast(2)
	eventasssert.OnStore(sink2).Match(eventMatcher).Not()
}
