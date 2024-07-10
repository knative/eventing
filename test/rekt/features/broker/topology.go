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
	"fmt"
	"strconv"

	conformanceevent "github.com/cloudevents/conformance/pkg/event"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/rekt/features/knconf"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	triggerresources "knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

type triggerCfg struct {
	failCount uint
	filter    *eventingv1.TriggerFilter
	delivery  *v1.DeliverySpec
	reply     *conformanceevent.Event
}

// createBrokerTriggerDeliveryTopology creates a topology that allows us to test the various
// delivery configurations.
//
// source ---> [broker (brokerDS)] --+--[trigger0 (ds, filter)]--> "t0" + (optional reply)
//
//	|         |              |
//	|         |              +--> "t0dlq" (optional)
//	|        ...
//	|         +-[trigger{n} (ds, filter)]--> "t{n}" + (optional reply)
//	|                       |
//	|                       +--> "t{n}dlq" (optional)
//	|
//	+--[DLQ]--> "dlq" (optional)
func createBrokerTriggerTopology(f *feature.Feature, brokerName string, brokerDS *v1.DeliverySpec, triggers []triggerCfg, brokerOpts ...manifest.CfgFn) *eventshub.EventProber {
	prober := eventshub.NewProber()

	f.Setup("install recorder for broker dlq", prober.ReceiverInstall("brokerdlq"))
	brokerOpts = append(brokerOpts, brokerresources.WithEnvConfig()...)

	if brokerDS != nil {
		if brokerDS.DeadLetterSink != nil {
			brokerOpts = append(brokerOpts, delivery.WithDeadLetterSink(prober.AsKReference("brokerdlq"), ""))
		}
		if brokerDS.Retry != nil {
			brokerOpts = append(brokerOpts, delivery.WithRetry(*brokerDS.Retry, brokerDS.BackoffPolicy, brokerDS.BackoffDelay))
		}
	}
	f.Setup("Create Broker", brokerresources.Install(brokerName, brokerOpts...))
	f.Setup("Broker is Ready", brokerresources.IsReady(brokerName)) // We want to block until broker is ready to go.

	prober.SetTargetResource(brokerresources.GVR(), brokerName)

	for i, t := range triggers {
		name := fmt.Sprintf("t%d", i)
		dlqName := fmt.Sprintf("t%ddlq", i)

		nameOpts := []eventshub.EventsHubOption{eventshub.DropFirstN(t.failCount)}
		if t.reply != nil {
			nameOpts = append(nameOpts,
				eventshub.ReplyWithTransformedEvent(t.reply.Attributes.Type, t.reply.Attributes.Source, t.reply.Data))
		}

		// TODO: Optimize these to only install things required. For example, if there's no tn dlq, no point creating a prober for it.
		f.Setup("install recorder for "+name, prober.ReceiverInstall(name, nameOpts...))
		f.Setup("install recorder for "+dlqName, prober.ReceiverInstall(dlqName))

		tOpts := []manifest.CfgFn{triggerresources.WithSubscriber(prober.AsKReference(name), ""), triggerresources.WithBrokerName(brokerName)}

		if t.delivery != nil {
			if t.delivery.DeadLetterSink != nil {
				tOpts = append(tOpts, delivery.WithDeadLetterSink(prober.AsKReference(dlqName), ""))
			}
			if t.delivery.Retry != nil {
				tOpts = append(tOpts, delivery.WithRetry(*t.delivery.Retry, t.delivery.BackoffPolicy, t.delivery.BackoffDelay))
			}
		}
		if t.filter != nil {
			tOpts = append(tOpts, triggerresources.WithFilter(t.filter.Attributes))
		}

		triggerName := feature.MakeRandomK8sName(name)
		f.Setup("Create Trigger"+strconv.Itoa(i)+" with recorder",
			triggerresources.Install(triggerName, tOpts...))

		f.Setup("Trigger"+strconv.Itoa(i)+" is ready",
			triggerresources.IsReady(triggerName))
	}
	return prober
}

// createExpectedEventMap creates a datastructure for a given test topology created by `createBrokerTriggerDeliveryTopology` function.
// Things we know from the DeliverySpecs passed in are where failed events from both t1 and t2 should land in.
// We also know how many events (incoming as well as how many failures the trigger subscriber is supposed to see).
// Note there are lot of baked assumptions and very tight coupling between this and `createBrokerTriggerDeliveryTopology` function.
func createExpectedEventPatterns(brokerDS *v1.DeliverySpec, triggers []triggerCfg) map[string]knconf.EventPattern {
	// By default, assume that nothing gets anything.
	p := map[string]knconf.EventPattern{
		"brokerdlq": {
			Success:  []bool{},
			Interval: []uint{},
		},
	}

	brokerdlq := false
	for i, t := range triggers {
		name := fmt.Sprintf("t%d", i)
		dlqName := fmt.Sprintf("t%ddlq", i)
		p[name] = knconf.EventPattern{
			Success:  []bool{},
			Interval: []uint{},
		}
		p[dlqName] = knconf.EventPattern{
			Success:  []bool{},
			Interval: []uint{},
		}

		attempts := knconf.DeliveryAttempts(t.delivery, brokerDS)

		p[name] = knconf.PatternFromEstimates(attempts, t.failCount)
		if attempts <= t.failCount {
			if t.delivery != nil && t.delivery.DeadLetterSink != nil {
				p[dlqName] = knconf.PatternFromEstimates(1, 0)
			} else {
				brokerdlq = true
			}
		}
		// TODO: replies do not work yet.
	}

	if brokerdlq && brokerDS != nil && brokerDS.DeadLetterSink != nil {
		p["brokerdlq"] = knconf.PatternFromEstimates(1, 0)
	}

	return p
}

func assertExpectedRoutedEvents(prober *eventshub.EventProber, expected map[string][]conformanceevent.Event) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		for prefix, want := range expected {
			got := knconf.ReceivedEventsFromProber(ctx, prober, prefix)

			t.Logf("Expected Events %s; \nGot: %#v\n Want: %#v", prefix, got, want)
			if len(want) != len(got) {
				t.Errorf("Wanted %d events, got %d", len(want), len(got))
			}

			// Check event acceptance.
			if len(want) != 0 && len(got) != 0 {
				// ID is adjusted by eventshub.
				except := []cmp.Option{
					cmpopts.IgnoreFields(conformanceevent.ContextAttributes{}, "ID", "Extensions"),
					cmpopts.IgnoreMapEntries(func(k, v string) bool { return k == "knativearrivaltime" }),
				}
				if diff := cmp.Diff(want, got, except...); diff != "" {
					t.Error("unexpected event routing behaviour (-want, +got) =", diff)
				}
			}
		}
	}
}

// createExpectedEventRoutingMap takes in an array of trigger configurations as well as incoming events and
// constructs a map of where the events should land. Any replies in trigger configurations will be treated
// as matchable events (since they are going to be sent back to the Broker).
// TODO: This function only handles replies generated to incoming events properly and it's fine for our
// tests for now. But if you wanted to test calling filter T0 which would generated EReply which would
// match filter T1 which would generate EReplyTwo, then that won't work.
func createExpectedEventRoutingMap(triggerConfigs []triggerCfg, inEvents []conformanceevent.Event) map[string][]conformanceevent.Event {
	ret := make(map[string][]conformanceevent.Event, len(triggerConfigs))

	repliesGenerated := make([]conformanceevent.Event, 0)

	// For each of the events (both incoming and newly created (replies)) check each trigger filter and append
	// to the expected events if it matches.
	for _, e := range inEvents {
		for i, config := range triggerConfigs {
			triggerName := fmt.Sprintf("t%d", i)
			if eventMatchesTrigger(e, config.filter) {
				ret[triggerName] = append(ret[triggerName], e)
				// Ok, so if there is a reply, and this trigger was tickled, add as "generated reply" event that's
				// used below.
				if config.reply != nil {
					repliesGenerated = append(repliesGenerated, *config.reply)
				}
			}
		}
	}

	for _, e := range repliesGenerated {
		for i, config := range triggerConfigs {
			triggerName := fmt.Sprintf("t%d", i)
			if eventMatchesTrigger(e, config.filter) {
				ret[triggerName] = append(ret[triggerName], e)
			}
		}
	}
	return ret
}

// eventNMatchesTrigger checks an event and returns True if the event matches the event.
// nil filter means everything matches, so it's safe to pass nil in here.
func eventMatchesTrigger(event conformanceevent.Event, filter *eventingv1.TriggerFilter) bool {
	// With no filter, everything matches
	if filter == nil {
		return true
	}
	for attribute, value := range filter.Attributes {
		switch attribute {
		case "type":
			return event.Attributes.Type == value
		case "source":
			return event.Attributes.Source == value
		case "subject":
			return event.Attributes.Subject == value
		}
		// Not a well known attribute, check extensions.
		filterAttribute, ok := event.Attributes.Extensions[attribute]
		if !ok {
			// We want an attribute on the event, but it's not there, so no soup for you.
			return false
		}
		return filterAttribute == attribute
	}
	// TODO: Do more matching here as necessary.
	return false
}
