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
	"context"
	"fmt"
	"strconv"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
	"knative.dev/reconciler-test/pkg/state"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/features/knconf"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/subscription"
)

func DataPlaneConformance(channelName string) *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Channel Specification - Data Plane",
		Features: []*feature.Feature{
			DataPlaneChannel(channelName),
		},
	}

	addControlPlaneDelivery(fs)

	return fs
}

func DataPlaneChannel(channelName string) *feature.Feature {
	f := feature.NewFeatureNamed("Conformance")

	f.Setup("Set Channel Name", setChannelableName(channelName))

	f.Requirement("Channel is Ready", channel_impl.IsReady(channelName))

	f.Stable("Input").
		Must("Every Channel MUST expose either an HTTP or HTTPS endpoint.", checkEndpoint).
		Must("The endpoint(s) MUST conform to 0.3 or 1.0 CloudEvents specification.",
			channelAcceptsCEVersions).
		MustNot("The Channel MUST NOT perform an upgrade of the passed in version. It MUST emit the event with the same version.", ShouldNotUpdateVersion).
		Must("It MUST support Binary Content Mode of the HTTP Protocol Binding for CloudEvents.", channelAcceptsBinaryContentMode).
		Must("It MUST support Structured Content Mode of the HTTP Protocol Binding for CloudEvents.", channelAcceptsStructuredContentMode).
		May("Channels MAY expose other, non-HTTP endpoints in addition to HTTP at their discretion.", checkEndpoints).
		May("When dispatching the event, the channel MAY use a different HTTP Message mode of the one used by the event.", todo).
		May("The HTTP(S) endpoint MAY be on any port, not just the standard 80 and 443.", todo)

	f.Stable("Generic").
		Must("If a Channel receives an event queueing request and is unable to parse a valid CloudEvent, then it MUST reject the request.", channelRejectsMalformedCE)

	f.Stable("HTTP").
		Must("Channels MUST reject all HTTP event queueing requests with a method other than POST responding with HTTP status code 405 Method Not Supported.", todo).
		Must("The HTTP event queueing request's URL MUST correspond to a single, unique Channel at any given moment in time.", todo).
		May("This MAY be done via the host, path, query string, or any combination of these.", todo).
		Must("If a Channel receives a request that does not correspond to a known channel, then it MUST respond with a 404 Not Found.", todo).
		Must("The Channel MUST respond with 202 Accepted if the event queueing request is accepted by the server.", todo).
		Must("If a Channel receives an event queueing request and is unable to parse a valid CloudEvent, then it MUST respond with 400 Bad Request.", todo)

	f.Stable("Output").
		Must("Channels MUST output CloudEvents.", todo).
		Must("The output MUST match the CloudEvent version of the Input.", todo).
		MustNot("Channels MUST NOT alter an event that goes through them.", todo).
		Must("All CloudEvent attributes, including the data attribute, MUST be received at the subscriber identical to how they were received by the Channel.", todo).
		Should("Every Channel SHOULD support sending events via Binary Content Mode or Structured Content Mode of the HTTP Protocol Binding for CloudEvents.", todo).
		Must("Channels MUST send events to all subscribers which are marked with a status of ready: True in the channel's status.subscribers.", todo).
		Must("The events MUST be sent to the subscriberURI field of spec.subscribers.", todo)
		// Each channel implementation will have its own quality of service guarantees (e.g. at least once, at most once, etc) which SHOULD be documented.

	f.Stable("Observability").
		Should("Channels SHOULD expose a variety of metrics: [Number of malformed incoming event queueing events (400 Bad Request responses)]", todo).
		Should("Channels SHOULD expose a variety of metrics: [Number of accepted incoming event queuing events (202 Accepted responses)]", todo).
		Should("Channels SHOULD expose a variety of metrics: [Number of egress CloudEvents produced (with the former metric, used to derive channel queue size)]", todo).
		Should("Metrics SHOULD be enabled by default, with a configuration parameter included to disable them if desired.", todo).
		Must("The Channel MUST recognize and pass through all tracing information from sender to subscribers using W3C Tracecontext.", todo).
		Should("The Channel SHOULD sample and write traces to the location specified in config-tracing.", todo).
		Should("Spans emitted by the Channel SHOULD follow the OpenTelemetry Semantic Conventions for Messaging Systems whenever possible.", todo).
		Should("Spans emitted by the Channel SHOULD set the knative attributes.", todo)
		// messaging.system: "knative"
		// messaging.destination: url to which the event is being routed
		// messaging.protocol: the name of the underlying transport protocol
		// messaging.message_id: the event ID

	f.Teardown("cleanup created resources", f.DeleteResources)

	return f
}

func checkEndpoint(ctx context.Context, t feature.T) {
	c := getChannelable(ctx, t)
	addr := *c.Status.AddressStatus.Address
	checkScheme(t, addr)
}

func checkEndpoints(ctx context.Context, t feature.T) {
	c := getChannelable(ctx, t)
	for _, addr := range c.Status.AddressStatus.Addresses {
		checkScheme(t, addr)
	}
}

func checkScheme(t feature.T, addr duckv1.Addressable) {
	if addr.URL == nil {
		addr.URL = new(apis.URL)
	}
	if addr.URL.Scheme != "http" && addr.URL.Scheme != "https" {
		t.Fatalf("expected channel scheme to be HTTP or HTTPS as addons with each other but got: %s", addr.URL.Scheme)
	}
}

func channelRejectsMalformedCE(ctx context.Context, t feature.T) {
	channelName := state.GetStringOrFail(ctx, t, ChannelableNameKey)

	headers := map[string]string{
		"ce-specversion": "1.0",
		"ce-type":        "sometype",
		"ce-source":      "conformancetest.request.sender.test.knative.dev",
		"ce-id":          uuid.New().String(),
	}

	for k := range headers {
		// Add all but the one key we want to omit.

		var options []eventshub.EventsHubOption
		for k2, v2 := range headers {
			if k != k2 {
				options = append(options, eventshub.InputHeader(k2, v2))
				t.Logf("Adding Header Value: %q => %q", k2, v2)
			}
		}
		options = append(options, eventshub.StartSenderToResource(channel_impl.GVR(), channelName))
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

func channelAcceptsCEVersions(ctx context.Context, t feature.T) {
	name := state.GetStringOrFail(ctx, t, ChannelableNameKey)
	knconf.AcceptsCEVersions(ctx, t, channel_impl.GVR(), name)
}

func addControlPlaneDelivery(fs *feature.FeatureSet) {
	// Should("Channels SHOULD retry resending CloudEvents when they fail to either connect or send CloudEvents to subscribers.", todo).
	// Should("Channels SHOULD support various retry configuration parameters: [the maximum number of retries]", todo).
	// Should("Channels SHOULD support various retry configuration parameters: [the time in-between retries]", todo).
	// Should("Channels SHOULD support various retry configuration parameters: [the backoff rate]", todo)

	for i, tt := range []struct {
		name string
		chDS *v1.DeliverySpec
		subs []subCfg
	}{{
		name: "Channels SHOULD retry resending CloudEvents when they fail to either connect or send CloudEvents to subscribers.",
		subs: []subCfg{{
			prefix:         "sub1",
			hasSub:         true,
			subFailCount:   1,
			subReplies:     false,
			hasReply:       false,
			replyFailCount: 0,
			delivery: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(1),
			},
		}},
	}, {
		name: "Channels SHOULD support various retry configuration parameters: [the maximum number of retries]",
		subs: []subCfg{{
			prefix:         "sub1",
			hasSub:         true,
			subFailCount:   1,
			subReplies:     false,
			hasReply:       false,
			replyFailCount: 0,
			delivery: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(1),
			},
		}, {
			prefix:         "sub2",
			hasSub:         true,
			subFailCount:   1,
			subReplies:     true,
			hasReply:       true,
			replyFailCount: 0,
			delivery: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(1),
			},
		}, {
			prefix:         "sub3",
			hasSub:         true,
			subFailCount:   1,
			subReplies:     true,
			hasReply:       true,
			replyFailCount: 1,
			delivery: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(1),
			},
		}, {
			prefix:         "sub4",
			hasSub:         true,
			subFailCount:   1,
			subReplies:     true,
			hasReply:       true,
			replyFailCount: 2,
			delivery: &v1.DeliverySpec{
				DeadLetterSink: new(duckv1.Destination),
				Retry:          ptr.Int32(1),
			},
		}},
	}} {
		f := feature.NewFeatureNamed("Delivery Spec " + strconv.Itoa(i))

		chName := fmt.Sprintf("dlq-test-%d", i)
		f.Setup("Set channel Name", setChannelableName(chName))

		prober := createChannelTopology(f, chName, tt.chDS, tt.subs)

		// Send an event into the matrix and hope for the best
		prober.SenderFullEvents(1)
		f.Requirement("install source", prober.SenderInstall("source"))

		// All events have been sent, time to look at the specs and confirm we got them.
		expected := createExpectedEventPatterns(tt.chDS, tt.subs)

		f.Requirement("wait until done", func(ctx context.Context, t feature.T) {
			interval, timeout := environment.PollTimingsFromContext(ctx)
			err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
				gtg := true
				for prefix, want := range expected {
					events := prober.ReceivedOrRejectedBy(ctx, prefix)
					if len(events) != len(want.Success) {
						gtg = false
					}
				}
				return gtg, nil
			})
			if err != nil {
				t.Failed()
			}
		})

		f.Stable("Conformance").Should(tt.name, knconf.AssertEventPatterns(prober, expected))

		// Add this feature to the feature set.
		fs.Features = append(fs.Features, f)
	}
}

func channelAcceptsBinaryContentMode(ctx context.Context, t feature.T) {
	channelName := "mychannelimpl"
	contenttypes := []string{
		"application/vnd.apache.thrift.binary",
		"application/xml",
		"application/json",
	}
	for _, contenttype := range contenttypes {
		source := feature.MakeRandomK8sName("source")
		eventshub.Install(source,
			eventshub.StartSenderToResource(channel_impl.GVR(), channelName),
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

func channelAcceptsStructuredContentMode(ctx context.Context, t feature.T) {
	channelName := "mychannelimpl"
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
	sink := feature.MakeRandomK8sName("sink")
	eventshub.Install(sink, eventshub.StartReceiver)(ctx, t)
	source := feature.MakeRandomK8sName("source")
	eventshub.Install(source,
		eventshub.StartSenderToResource(channel_impl.GVR(), channelName),
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

func ShouldNotUpdateVersion(ctx context.Context, t feature.T) {
	f := feature.NewFeature()
	c := getChannelable(ctx, t)
	source := feature.MakeRandomK8sName("source")
	sub := feature.MakeRandomK8sName("subscription")
	sink := feature.MakeRandomK8sName("sink")

	event := test.FullEvent()

	event.SetSpecVersion("0.3")

	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(c.Name)),
		subscription.WithSubscriber(service.AsKReference(sink), "", ""),
	))

	f.Setup("ready", channel_impl.IsReady(c.Name))

	eventshub.Install(sink, eventshub.StartReceiver)

	eventshub.Install(source,
		eventshub.StartSenderToResource(channel_impl.GVR(), c.Name),
		eventshub.InputEvent(event),
	)(ctx, t)

	eventasssert.OnStore(sink).MatchEvent(test.HasSpecVersion("0.3")).AtLeast(1)
}
