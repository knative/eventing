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

	"k8s.io/apimachinery/pkg/util/wait"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/features/knconf"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
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
		Must("Every Channel MUST expose either an HTTP or HTTPS endpoint.", todo).
		Must("The endpoint(s) MUST conform to 0.3 or 1.0 CloudEvents specification.",
			channelAcceptsCEVersions).
		MustNot("The Channel MUST NOT perform an upgrade of the passed in version. It MUST emit the event with the same version.", todo).
		Must("It MUST support both Binary Content Mode and Structured Content Mode of the HTTP Protocol Binding for CloudEvents.", todo).
		May("When dispatching the event, the channel MAY use a different HTTP Message mode of the one used by the event.", todo).
		// For example, It MAY receive an event in Structured Content Mode and dispatch in Binary Content Mode.
		May("The HTTP(S) endpoint MAY be on any port, not just the standard 80 and 443.", todo).
		May("Channels MAY expose other, non-HTTP endpoints in addition to HTTP at their discretion.", todo)
		// (e.g. expose a gRPC endpoint to accept events)

	f.Stable("Generic").
		Must("If a Channel receives an event queueing request and is unable to parse a valid CloudEvent, then it MUST reject the request.", todo)

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

	return f
}

func channelAcceptsCEVersions(ctx context.Context, t feature.T) {
	name := state.GetStringOrFail(ctx, t, ChannelableNameKey)
	knconf.AcceptsCEVersions(ctx, t, channel_impl.GVR(), name)
}

func addControlPlaneDelivery(fs *feature.FeatureSet) {
	//Should("Channels SHOULD retry resending CloudEvents when they fail to either connect or send CloudEvents to subscribers.", todo).
	//Should("Channels SHOULD support various retry configuration parameters: [the maximum number of retries]", todo).
	//Should("Channels SHOULD support various retry configuration parameters: [the time in-between retries]", todo).
	//Should("Channels SHOULD support various retry configuration parameters: [the backoff rate]", todo)

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
		f.Setup("Set Broker Name", setChannelableName(chName))

		prober := createChannelTopology(f, chName, tt.chDS, tt.subs)

		// Send an event into the matrix and hope for the best
		prober.SenderFullEvents(1)
		f.Setup("install source", prober.SenderInstall("source"))
		f.Requirement("sender is finished", prober.SenderDone("source"))

		// All events have been sent, time to look at the specs and confirm we got them.
		expected := createExpectedEventPatterns(tt.chDS, tt.subs)

		f.Requirement("wait until done", func(ctx context.Context, t feature.T) {
			interval, timeout := environment.PollTimingsFromContext(ctx)
			err := wait.PollImmediate(interval, timeout, func() (bool, error) {
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
