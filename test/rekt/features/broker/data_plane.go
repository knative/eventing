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

	"github.com/google/uuid"
	"knative.dev/eventing/test/rekt/features/knconf"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/pkg/apis"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
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
			icebox).
		Should("The Broker SHOULD attempt to notify the operator in this case.",
			icebox).
		May("The Broker MAY forward these events to an alternate endpoint or storage mechanism such as a dead letter queue.",
			icebox).
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
		events := knconf.Correlate(store.AssertAtLeast(ctx, t, 2, knconf.SentEventMatcher("")))
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
	events := knconf.Correlate(store.AssertAtLeast(ctx, t, 2, knconf.SentEventMatcher("")))
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
		events := knconf.Correlate(store.AssertAtLeast(ctx, t, 2, knconf.SentEventMatcher("")))
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
		events := knconf.Correlate(store.AssertAtLeast(ctx, t, 2, knconf.SentEventMatcher("")))
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
		events := knconf.Correlate(store.AssertAtLeast(ctx, t, 2, knconf.SentEventMatcher("")))
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
		events := knconf.Correlate(store.AssertAtLeast(ctx, t, 2, knconf.SentEventMatcher("")))
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
