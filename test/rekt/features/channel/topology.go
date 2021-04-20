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
	"sort"
	"strconv"

	"github.com/google/go-cmp/cmp"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

type subCfg struct {
	prefix         string
	hasSub         bool
	subFailCount   uint
	subReplies     bool
	hasReply       bool
	replyFailCount uint
	delivery       *v1.DeliverySpec
}

func (s *subCfg) subName() string {
	suffix := "sub"
	return fmt.Sprintf("%s%s", s.prefix, suffix)
}

func (s *subCfg) replyName() string {
	suffix := "reply"
	return fmt.Sprintf("%s%s", s.prefix, suffix)
}

func (s *subCfg) dlqName() string {
	suffix := "dlq"
	return fmt.Sprintf("%s%s", s.prefix, suffix)
}

// createChannelTopology creates a channel and {n} subscriptions with recorders
// attached to each endpoint.
//
//  source --> [channel (chDS)] --+--[sub1 (sub1DS)]--> sub1sub (optional) --> sub1reply (optional)
//                        |       |            |
//                        |       |            +-->sub1dlq (optional)
//                        |      ...
//                        |       +-[sub{n} (sub{n}DS)]--> sub{n}sub (optional)--> sub{n}reply (optional)
//                        |                   |
//                        |                   +-->sub{n}dlq (optional)
//                        |
//                        +--[DLQ]--> chdlq (optional)
//
func createChannelTopology(f *feature.Feature, chName string, chDS *v1.DeliverySpec, subs []subCfg) *eventshub.EventProber {
	prober := eventshub.NewProber()
	// Install the receivers.
	f.Setup("install channel DLQ", prober.ReceiverInstall("chdlq"))

	var chOpts []manifest.CfgFn
	if chDS != nil {
		if chDS.DeadLetterSink != nil {
			chOpts = append(chOpts, delivery.WithDeadLetterSink(prober.AsKReference("chdlq"), ""))
		}
		if chDS.Retry != nil {
			chOpts = append(chOpts, delivery.WithRetry(*chDS.Retry, chDS.BackoffPolicy, chDS.BackoffDelay))
		}
	}
	f.Setup("Create Channel Impl", channel_impl.Install(chName, chOpts...))
	f.Setup("Channel is Ready", channel_impl.IsReady(chName)) // We want to block in the setup phase until the channel is ready to go.

	// Set the prober target.
	prober.SetTargetResource(channel_impl.GVR(), chName)

	// Install subscriptions.
	for i, sub := range subs {
		// Install the expected sinks, they all might not be used.

		subOpts := []eventshub.EventsHubOption{eventshub.DropFirstN(sub.subFailCount)}
		if sub.subReplies {
			subOpts = append(subOpts, eventshub.ReplyWithAppendedData(sub.prefix))
		}
		f.Setup("install subscription"+strconv.Itoa(i)+" subscriber",
			prober.ReceiverInstall(sub.subName(), subOpts...))

		f.Setup("install subscription"+strconv.Itoa(i)+" reply",
			prober.ReceiverInstall(sub.replyName(), eventshub.DropFirstN(sub.replyFailCount)))
		f.Setup("install subscription"+strconv.Itoa(i)+" DLQ",
			prober.ReceiverInstall(sub.dlqName()))

		var opts []manifest.CfgFn
		if sub.hasSub {
			opts = append(opts, subscription.WithSubscriber(prober.AsKReference(sub.subName()), ""))
		}

		if sub.hasReply {
			opts = append(opts, subscription.WithReply(prober.AsKReference(sub.replyName()), ""))
		}

		if sub.delivery.DeadLetterSink != nil {
			if sub.delivery != nil {
				opts = append(opts, subscription.WithDeadLetterSink(prober.AsKReference(sub.dlqName()), ""))
			}
			if sub.delivery.Retry != nil {
				opts = append(opts, subscription.WithRetry(*sub.delivery.Retry, sub.delivery.BackoffPolicy, sub.delivery.BackoffDelay))
			}
		}
		opts = append(opts, subscription.WithChannel(channel_impl.AsRef(chName)))
		name := feature.MakeRandomK8sName(sub.prefix)
		f.Setup("install subscription"+strconv.Itoa(i), subscription.Install(name, opts...))
		f.Setup("subscription"+strconv.Itoa(i)+" is ready", subscription.IsReady(name))
	}

	return prober
}

type eventPattern struct {
	success  []bool
	interval []uint
}

func makeEventPattern(attempts, failures uint) eventPattern {
	p := eventPattern{
		success:  []bool{},
		interval: []uint{},
	}
	for i := uint(0); i < attempts; i++ {
		if i >= failures {
			p.success = append(p.success, true)
			p.interval = append(p.interval, 0) // TODO: calculate time.
			return p
		}
		p.success = append(p.success, false)
		p.interval = append(p.interval, 0) // TODO: calculate time.
	}
	return p
}

// createExpectedEventPatterns assumes a single event is sent.
func createExpectedEventPatterns(chDS *v1.DeliverySpec, subs []subCfg) map[string]eventPattern {
	// By default, assume that nothing gets anything.
	p := map[string]eventPattern{
		"chdlq": {
			success:  []bool{},
			interval: []uint{},
		},
	}

	chdlq := false

	for _, sub := range subs {
		p[sub.subName()] = eventPattern{
			success:  []bool{},
			interval: []uint{},
		}
		p[sub.replyName()] = eventPattern{
			success:  []bool{},
			interval: []uint{},
		}
		p[sub.dlqName()] = eventPattern{
			success:  []bool{},
			interval: []uint{},
		}

		skipReply := false
		attempts := deliveryAttempts(sub.delivery, chDS)
		// For subscriber.
		if sub.hasSub {
			p[sub.subName()] = makeEventPattern(attempts, sub.subFailCount)
			if attempts <= sub.subFailCount {
				skipReply = true
				if sub.delivery != nil && sub.delivery.DeadLetterSink != nil {
					p[sub.dlqName()] = makeEventPattern(1, 0)
				} else {
					chdlq = true
				}
			}
			if !sub.subReplies {
				skipReply = true
			}
		}
		// For reply.
		if !skipReply && sub.hasReply {
			p[sub.replyName()] = makeEventPattern(attempts, sub.replyFailCount)
			if attempts <= sub.replyFailCount {
				if sub.delivery != nil && sub.delivery.DeadLetterSink != nil {
					p[sub.dlqName()] = makeEventPattern(1, 0)
				} else {
					chdlq = true
				}
			}
		}
	}
	if chdlq && chDS != nil && chDS.DeadLetterSink != nil {
		p["chdlq"] = makeEventPattern(1, 0)
	}

	return p
}

func deliveryAttempts(p0, p1 *v1.DeliverySpec) uint {
	if p0 != nil {
		if p0.Retry == nil || *p0.Retry == 0 {
			return 1
		}
		return 1 + uint(*p0.Retry)
	}

	if p1 != nil {
		if p1.Retry == nil || *p1.Retry == 0 {
			return 1
		}
		return 1 + uint(*p1.Retry)
	}

	return 1
}

func assertExpectedEvents(prober *eventshub.EventProber, expected map[string]eventPattern) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		for prefix, want := range expected {
			got := happened(ctx, prober, prefix)

			t.Logf("Expected Events %s; \n Got: %#v\nWant: %#v", prefix, got, want)

			// Check event acceptance.
			if len(want.success) != 0 && len(got.success) != 0 {
				if diff := cmp.Diff(want.success, got.success); diff != "" {
					t.Error("unexpected event acceptance behaviour (-want, +got) =", diff)
				}
			}
			// Check timing.
			//if len(want.eventInterval) != 0 && len(got.eventInterval) != 0 {
			//	if diff := cmp.Diff(want.eventInterval, got.eventInterval); diff != "" {
			//		t.Error("unexpected event interval behaviour (-want, +got) =", diff)
			//	}
			//}
		}
	}
}

// TODO: this function could be moved to the prober directly.
func happened(ctx context.Context, prober *eventshub.EventProber, prefix string) eventPattern {
	events := prober.ReceivedOrRejectedBy(ctx, prefix)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})
	got := eventPattern{
		success:  make([]bool, 0),
		interval: make([]uint, 0),
	}
	for i, event := range events {
		got.success = append(got.success, event.Kind == eventshub.EventReceived)
		if i == 0 {
			got.interval = []uint{0}
		} else {
			diff := events[i-1].Time.Unix() - event.Time.Unix()
			got.interval = append(got.interval, uint(diff))
		}
	}
	return got
}
