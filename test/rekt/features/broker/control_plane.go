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
	"encoding/json"
	"fmt"
	"strings"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/features/knconf"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/svc"
	triggerresources "knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
)

func ControlPlaneConformance(brokerName string) *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Broker Specification - Control Plane",
		Features: []feature.Feature{
			*ControlPlaneBroker(brokerName),
			*ControlPlaneTrigger_GivenBroker(brokerName),
			*ControlPlaneTrigger_GivenBrokerTriggerReady(brokerName),
			*ControlPlaneTrigger_WithBrokerLifecycle(),
			*ControlPlaneTrigger_WithValidFilters(brokerName),
			*ControlPlaneTrigger_WithInvalidFilters(brokerName),
			*ControlPlaneDelivery(brokerName),
		},
	}
	// TODO: This is not a control plane test, or at best it is a blend with data plane.
	// Must("Events that pass the attributes filter MUST include context or extension attributes that match all key-value pairs exactly.", todo)

	return fs
}

func setBrokerName(name string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, BrokerNameKey, name)
	}
}

func ControlPlaneBroker(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Broker")

	f.Setup("Set Broker Name", setBrokerName(brokerName))

	f.Stable("Conformance").
		Should("Broker objects SHOULD include a Ready condition in their status",
			knconf.KResourceHasReadyInConditions(brokerresources.GVR(), brokerName)).
		Should("The Broker SHOULD indicate Ready=True when its ingress is available to receive events.",
			readyBrokerHasIngressAvailable).
		Should("While a Broker is Ready, it SHOULD be a valid Addressable and its `status.address.url` field SHOULD indicate the address of its ingress.",
			readyBrokerIsAddressable).
		Should("The class of a Broker object SHOULD be immutable.",
			brokerClassIsImmutable)
	return f
}

func ControlPlaneTrigger_GivenBroker(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Trigger, Given Broker")
	f.Setup("Set Broker Name", setBrokerName(brokerName))

	subscriberName := feature.MakeRandomK8sName("sub")
	f.Setup("Install Subscriber", svc.Install(subscriberName, "bad", "svc"))

	triggerName := feature.MakeRandomK8sName("trigger")
	f.Setup("Create a Trigger", triggerresources.Install(triggerName, brokerName,
		triggerresources.WithSubscriber(svc.AsRef(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, TriggerNameKey, triggerName)
	})

	f.Stable("Conformance").
		Should("Triggers SHOULD include a Ready condition in their status.",
			triggerHasReadyInConditions).
		Should("The Trigger SHOULD indicate Ready=True when events can be delivered to its subscriber.",
			readyTriggerCanDeliver).
		Must("Triggers MUST be assigned to exactly one Broker.",
			triggerHasOneBroker).
		Must("The assigned Broker of a Trigger SHOULD be immutable.",
			triggerSpecBrokerIsImmutable)

	return f
}

func ControlPlaneTrigger_GivenBrokerTriggerReady(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Trigger, Given Broker")
	f.Setup("Set Broker Name", setBrokerName(brokerName))

	subscriberName := feature.MakeRandomK8sName("sub")
	f.Setup("Install Subscriber", svc.Install(subscriberName, "bad", "svc"))

	triggerName := feature.MakeRandomK8sName("trigger")
	f.Setup("Create a Trigger", triggerresources.Install(triggerName, brokerName,
		triggerresources.WithSubscriber(svc.AsRef(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, TriggerNameKey, triggerName)
	})

	f.Requirement("The Trigger is Ready", triggerresources.IsReady(triggerName))

	f.Stable("Conformance").
		Should("While a Trigger is Ready, it SHOULD indicate its subscriber's URI via the `status.subscriberUri` field.",
			readyTriggerHasSubscriberURI)

	return f
}

func ControlPlaneTrigger_WithBrokerLifecycle() *feature.Feature {
	f := feature.NewFeatureNamed("Trigger, With Broker Lifecycle")

	subscriberName := feature.MakeRandomK8sName("sub")
	f.Setup("Install Subscriber", svc.Install(subscriberName, "bad", "svc"))

	brokerName := feature.MakeRandomK8sName("broker")

	triggerName := feature.MakeRandomK8sName("trigger")
	f.Setup("Create a Trigger", triggerresources.Install(triggerName, brokerName,
		triggerresources.WithSubscriber(svc.AsRef(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, TriggerNameKey, triggerName)
	})

	f.Stable("Conformance").
		May("A Trigger MAY be created before its assigned Broker exists.",
			triggerHasOneBroker).
		Should("A Trigger SHOULD progress to Ready when its assigned Broker exists and is Ready.",
			func(ctx context.Context, t feature.T) {
				brokerresources.Install(brokerName, brokerresources.WithEnvConfig()...)(ctx, t) // Default broker from Env.
				brokerresources.IsReady(brokerName)(ctx, t)
				triggerresources.IsReady(triggerName)(ctx, t)
			})
	return f
}

func ControlPlaneTrigger_WithValidFilters(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Trigger, With Filters")
	f.Setup("Set Broker Name", setBrokerName(brokerName))

	subscriberName := feature.MakeRandomK8sName("sub")
	f.Setup("Install Subscriber", svc.Install(subscriberName, "bad", "svc"))

	// CloudEvents attribute names MUST consist of lower-case letters ('a' to 'z') or digits ('0' to '9') from the ASCII character set. Attribute names SHOULD be descriptive and terse and SHOULD NOT exceed 20 characters in length.
	filters := map[string]string{
		"source":               "a source",
		"id":                   "an id",
		"specversion":          "the spec version",
		"type":                 "the type",
		"subject":              "a subject",
		"time":                 "a time",
		"datacontenttype":      "a datacontenttype",
		"dataschema":           "a dataschema",
		"aaa":                  "bbb",
		"c1d2e3":               "123",
		"abcdefghijklmnopqrst": "max length",
	}

	triggerName := feature.MakeRandomK8sName("trigger")
	f.Setup("Create a Trigger", triggerresources.Install(triggerName, brokerName,
		triggerresources.WithSubscriber(svc.AsRef(subscriberName), ""),
		triggerresources.WithFilter(filters),
	))

	f.Setup("Set Trigger Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, TriggerNameKey, triggerName)
	})

	f.Stable("Conformance").
		Must("The attributes filter specifying a list of key-value pairs MUST be supported by Trigger.",
			// Compare the passed filters with what is found on the control plane.
			func(ctx context.Context, t feature.T) {
				trigger := getTrigger(ctx, t)
				got := make(map[string]string)
				for k, v := range trigger.Spec.Filter.Attributes {
					got[k] = v
				}
				want := filters
				if diff := cmp.Diff(want, got, cmpopts.SortMaps(func(a, b string) bool {
					return a < b
				})); diff != "" {
					t.Error("Filters do not match (-want, +got) =", diff)
				}
			})

	return f
}

func ControlPlaneTrigger_WithInvalidFilters(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Trigger, With Filters")
	f.Setup("Set Broker Name", setBrokerName(brokerName))

	subscriberName := feature.MakeRandomK8sName("sub")
	f.Setup("Install Subscriber", svc.Install(subscriberName, "bad", "svc"))

	// CloudEvents attribute names MUST consist of lower-case letters ('a' to 'z') or digits ('0' to '9') from the ASCII character set. Attribute names SHOULD be descriptive and terse and SHOULD NOT exceed 20 characters in length.
	filters := map[string]string{
		"SOURCE":              "not lower case letters, all",
		"Source":              "not lower case letters, first",
		"souRce":              "not lower case letters, not first",
		"s pace s":            "no spaces",
		"s_pace_s":            "no underscores",
		"s-pace-s":            "no dashes",
		"123":                 "just numbers",
		"😊":                   "unicode not supported",
		"!@#$%^&*()-_=_`~+\\": "other non-(a-z,0-9) type chars, top row",
		"{}[];':\"<>,./?":     "other non-(a-z,0-9) type chars, brackets",
	}

	triggerName := feature.MakeRandomK8sName("trigger")
	f.Setup("Create a Trigger", triggerresources.Install(triggerName, brokerName,
		triggerresources.WithSubscriber(svc.AsRef(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, TriggerNameKey, triggerName)
	})

	asserter := f.Stable("Conformance - Negatives - The attributes filter specifying a list of key-value pairs MUST be supported by Trigger.")

	for key, value := range filters {
		k := key
		v := value
		asserter.Must("Reject invalid filter - "+k+" - "+v,
			// Compare the passed filters with what is found on the control plane.
			func(ctx context.Context, t feature.T) {
				trigger := getTrigger(ctx, t)

				if trigger.Spec.Filter == nil {
					trigger.Spec.Filter = &eventingv1.TriggerFilter{
						Attributes: map[string]string{},
					}
				} else if trigger.Spec.Filter.Attributes == nil {
					trigger.Spec.Filter.Attributes = map[string]string{}
				}

				trigger.Spec.Filter.Attributes[k] = v

				_, err := Client(ctx).Triggers.Update(ctx, trigger, metav1.UpdateOptions{})
				if err != nil {
					// We expect an error.
					// Success!
				} else {
					t.Error("expected Trigger to reject the spec.filter update.")
				}
			})
	}
	return f
}

func ControlPlaneDelivery(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Delivery Spec")

	f.Setup("Set Broker Name", setBrokerName(brokerName))

	// Make three brokers.

	// tabletest
	for i, tt := range []struct {
		name     string
		brokerDS *v1.DeliverySpec
		// Trigger 1 Delivery spec
		t1DS     *v1.DeliverySpec
		// How many events to fail before succeeding
		t1FailCount int
		// Trigger 2 Delivery spec
		t2DS     *v1.DeliverySpec
		// How many events to fail before succeeding
		t2FailCount int
	}{{
		name: "When `BrokerSpec.Delivery` and `TriggerSpec.Delivery` are both not configured, no delivery spec SHOULD be used.",
	}} {
		brokerName := fmt.Sprintf("dlq-test-%d", i)
		prober := createBrokerTriggerDeliveryTopology(f, brokerName, tt.brokerDS, tt.t1DS, tt.t2DS)

		// Send an event into the matrix and hope for the best
		prober.SenderFullEvents(1)
		f.Setup("install source", prober.SenderInstall("source"))
		f.Requirement("sender is finished", prober.SenderDone("source"))

		// All events have been sent, time to look at the specs and confirm we got them.
		assertBrokerTriggerDeliverySpec(prober, tt.brokerDS, tt.t1DS, "t1")
		assertBrokerTriggerDeliverySpec(prober, tt.brokerDS, tt.t1DS, "t1dlq")

		// ...Simple test case...
		// Broker has no retries


		expectedDeliverySpec


		expectedEvents := buildMap(tt.BrokerDS, tt.T1DS, tt.t2DS)
		// Helper that takes in: Delivery Specs => Returns "Places to Look [t1 => assertFunction, t1dlq => assertFunction, brokerdlq => assertFunction]
		// "t1" => {6 events, 6 failures, 0 success, linear}
		// "t1dlq" => {1 event}
		// "t2" => {6 events, 6 failures, 0 success exponential}
		// "t2dlq" => {0 events}
		// "brokerdql" => {1 event}

		// "t1" => {6 events, 5 failures, 1 success, linear}
		// "t1dlq" => {0 event}
		// "t2" => {6 events, 6 failures, 0 success exponential}
		// "t2dlq" => {0 events}
		// "brokerdql" => {1 event}

		assertEvents(expectedEvents[])
	    assertEvents("t1", "6 events", linear)

		// Make sure that Trigger 1 respected linear back, and failed 6 times
		assertEventsSeen("t1", helpers.SawLinearBackoff(tt.t1DS))
		// After 6 failures, was sent to DLQ
		prober.AssertReceivedAll("{key in the map", mapEntry)

		// Make sure that Trigger 2 respected linear back, and failed 6 times
		assertEventsSeen("t1", helpers.SawLinearBackoff(tt.t1DS))
		// After 6 failures, was sent to DLQ
		prober.AssertReceivedAll("t1dlq", )


		assertBrokerTriggerDeliverySpec(prober, tt.brokerDS, tt.t2DS, "t2")
		assertBrokerTriggerDeliverySpec(prober, tt.brokerDS, tt.t2DS, "t2dlq")
		assertBrokerTriggerDeliverySpec(prober, tt.brokerDS, tt.t2DS, "brokerdlq")

		assertBrokerXXX(prober, tt.t1Ds, "t1")
		assertBrokerXXX(prober, tt.t1Ds, "t1")

/*
	f.Setup("install recorder for t1", prober.ReceiverInstall("t1"))
	f.Setup("install recorder for t1dlq", prober.ReceiverInstall("t1dlq"))
	f.Setup("install recorder for t2", prober.ReceiverInstall("t2"))
	f.Setup("install recorder for t2dlq", prober.ReceiverInstall("t2dlq"))
	f.Setup("install recorder for broker dlq", prober.ReceiverInstall("brokerdlq"))
	*/		

		//f.Stable("Conformance").
		//	Should(tt.name,
		//		brokerTriggerDeliverySpec(prober, brokerDS, t1DS, t2DS))
	}
	//
	//f.Stable("Conformance").
	//	Should("When `BrokerSpec.Delivery` and `TriggerSpec.Delivery` are both not configured, no delivery spec SHOULD be used.",
	//		brokerTriggerDeliverySpec(brokerDS, t1DS, t2DS)).
	//	Should("When `BrokerSpec.Delivery` is configured, but not the specific `TriggerSpec.Delivery`, then the `BrokerSpec.Delivery` SHOULD be used.",
	//		brokerTriggerDeliverySpec(brokerDS, t1DS, t2DS)).
	//	Should("When `TriggerSpec.Delivery` is configured, then `TriggerSpec.Delivery` SHOULD be used.",
	//		brokerTriggerDeliverySpec(brokerDS, t1DS, t2DS)).
	//	Should("When both `BrokerSpec.Delivery` and `TriggerSpec.Delivery` is configured, then `TriggerSpec.Delivery` SHOULD be used.",
	//		brokerTriggerDeliverySpec(brokerDS, t1DS, t2DS))

	return f
}

type EventingClient struct {
	Brokers  eventingclientsetv1.BrokerInterface
	Triggers eventingclientsetv1.TriggerInterface
}

func Client(ctx context.Context) *EventingClient {
	ec := eventingclient.Get(ctx).EventingV1()
	env := environment.FromContext(ctx)

	return &EventingClient{
		Brokers:  ec.Brokers(env.Namespace()),
		Triggers: ec.Triggers(env.Namespace()),
	}
}

const (
	BrokerNameKey  = "brokerName"
	TriggerNameKey = "triggerName"
)

func getBroker(ctx context.Context, t feature.T) *eventingv1.Broker {
	c := Client(ctx)
	name := state.GetStringOrFail(ctx, t, BrokerNameKey)

	broker, err := c.Brokers.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get Broker, %v", err)
	}
	return broker
}

func getTrigger(ctx context.Context, t feature.T) *eventingv1.Trigger {
	c := Client(ctx)
	name := state.GetStringOrFail(ctx, t, TriggerNameKey)

	trigger, err := c.Triggers.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get Trigger, %v", err)
	}
	return trigger
}

func readyBrokerHasIngressAvailable(ctx context.Context, t feature.T) {
	// TODO: I am not sure how to test this from the outside.
}

func readyBrokerIsAddressable(ctx context.Context, t feature.T) {
	broker := getBroker(ctx, t)

	if broker.Status.IsReady() {
		if broker.Status.Address.URL == nil {
			t.Errorf("broker is not addressable")
		}
		// Success!
	} else {
		t.Errorf("broker was not ready")
	}
}

func brokerClassIsImmutable(ctx context.Context, t feature.T) {
	broker := getBroker(ctx, t)

	if broker.Annotations == nil {
		broker.Annotations = map[string]string{}
	}
	// update annotations
	broker.Annotations[eventingv1.BrokerClassAnnotationKey] = "Rekt.brokerClassIsImmutable"

	if _, err := Client(ctx).Brokers.Update(ctx, broker, metav1.UpdateOptions{}); err != nil {
		// Success!
		t.Log("broker class is immutable")
	} else {
		t.Errorf("broker class is mutable")
	}
}

func triggerHasReadyInConditions(ctx context.Context, t feature.T) {
	var trigger *eventingv1.Trigger

	interval, timeout := environment.PollTimingsFromContext(ctx)
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		trigger = getTrigger(ctx, t)
		if trigger.Status.ObservedGeneration != 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Errorf("unable to get a reconciled Trigger (status.observedGeneration != 0)")
	}

	knconf.HasReadyInConditions(ctx, t, trigger.Status.Status)
}

func readyTriggerCanDeliver(ctx context.Context, t feature.T) {
	trigger := getTrigger(ctx, t)
	_ = trigger
	// TODO: I am not sure how to test this from the outside.
}

func readyTriggerHasSubscriberURI(ctx context.Context, t feature.T) {
	trigger := getTrigger(ctx, t)
	if trigger.Status.IsReady() {
		if trigger.Status.SubscriberURI == nil {
			t.Errorf("trigger did not have subscriber uri in status")
		}
		// Success!
	} else {
		j, _ := json.Marshal(trigger)
		t.Errorf("trigger was not ready, \n%s", string(j))
	}
}

func triggerHasOneBroker(ctx context.Context, t feature.T) {
	trigger := getTrigger(ctx, t)
	if trigger.Spec.Broker == "" {
		t.Error("broker is empty")
	}
	if strings.Contains(trigger.Spec.Broker, ",") {
		t.Errorf("more than one broker specified: %q", trigger.Spec.Broker)
	}
}

func triggerSpecBrokerIsImmutable(ctx context.Context, t feature.T) {
	trigger := getTrigger(ctx, t)

	// Update spec.broker
	trigger.Spec.Broker = "Rekt.BrokerImmutable"

	if _, err := Client(ctx).Triggers.Update(ctx, trigger, metav1.UpdateOptions{}); err != nil {
		// Success!
		t.Log("Trigger spec.broker is immutable")
	} else {
		t.Errorf("Trigger spec.broker is mutable")
	}
}

// source ---> [broker (brokerDS)] ---[trigger(ds)]--> `tSink`
//                    |                      |
//                    |                      +--> `tDLQ` (optional)
//                    |
//                    + --> "dlq" (optional)
// Given that we know:
// 1. Which event we sent
// 2. What the topology looks like
// We should then be able to check which recorder (tPrefix) received which events.
// Take in a magic picture and make sure that all the pieces saw the events they were supposed to.
func assertBrokerTriggerDeliverySpec(ctx context.Context, prober *eventshub.EventProber, brokerDS, trigger1DS, trigger2DS *v1.DeliverySpec, tPrefix string) feature.Feature {

	// one or both brokerDS

	f.Stable("broker with DLQ").
		Must("accepted all events", prober.AssertSentAll("source")).
		Must("deliver event to DLQ (via1)", prober.AssertReceivedAll("source", tPrefix+"dlq")).
		Must("deliver event to sink (via2)", prober.AssertReceivedAll("source", "sink2"))

		// AssertReceivedAll tests that all events sent by `fromPrefix` were received by `toPrefix`.

	sent := p.SentBy(ctx, toPrefix)
	ids := make([]string, len(sent))
	for i, s := range sent {
		ids[i] = s.Sent.SentId
	}

	events := p.ReceivedBy(ctx, tPrefix)
	if len(ids) != len(events) {
		t.Errorf("expected %q to have received %d events, actually received %d",
			fromPrefix, len(ids), len(events))
	}
	for _, id := range ids {
		found := false
		for _, event := range events {
			if id == event.SentId {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Failed to receive event id=%s", id)
		}
	}

	return todo
}

//
// createBrokerTriggerDeliveryTopology creates a topology that allows us to test the various
// delivery configurations.
//
// source ---> [broker (brokerDS)] --+--[trigger1 (t1ds)]--> "t1"
//                  |                |                 |
//                  |                |                 +--> "t1dlq" (optional)
//                  |                |
//                  |                +-[trigger2 (t2ds)]--> "t2"
//                  |                |              |
//                  |                |              +--> "t2dlq" (optional)
//                  |                |
//                  +--[DLQ]--> "dlq" (optional)
//
func createBrokerTriggerDeliveryTopology(f *feature.Feature, brokerName string, brokerDS, t1DS, t2DS *v1.DeliverySpec, t1FailCount, t2FailCount int) *eventshub.EventProber {
	prober := eventshub.NewProber()
	// This will set or clear the broker delivery spec settings.
	// Make trigger with delivery settings.
	// Make a trigger with no delivery spec.

	// TODO: Optimize these to only install things required. For example, if there's no t2 dlq, no point creating a prober for it.
	f.Setup("install recorder for t1", prober.ReceiverInstall("t1")) // (wire in t1FailCount)
	f.Setup("install recorder for t1dlq", prober.ReceiverInstall("t1dlq"))
	f.Setup("install recorder for t2", prober.ReceiverInstall("t2")) // (wire in t2FailCount)
	f.Setup("install recorder for t2dlq", prober.ReceiverInstall("t2dlq"))
	f.Setup("install recorder for broker dlq", prober.ReceiverInstall("brokerdlq"))

	brokerOpts := brokerresources.WithEnvConfig()

	if brokerDS != nil {
		// TODO: fix this crap
		// With URI, or with retries, etc.
		brokerOpts = append(brokerOpts, delivery.WithDeadLetterSink(prober.AsKReference("brokerdlq"), ""))
	}
	f.Setup("Create Broker", brokerresources.Install(brokerName, brokerOpts...))

	if t1DS != nil {
		f.Setup("Create Trigger1 with recorder", triggerresources.Install("t1", brokerName,
			triggerresources.WithSubscriber(prober.AsKReference("t1"), "")))
	} else {
		f.Setup("Create Trigger1 with recorder", triggerresources.Install("t1", brokerName,
			triggerresources.WithSubscriber(prober.AsKReference("t1"), ""),
			delivery.WithDeadLetterSink(prober.AsKReference("t1dlq"), "")))
	}

	if t2DS != nil {
		f.Setup("Create Trigger2 with recorder", triggerresources.Install("t2", brokerName,
			triggerresources.WithSubscriber(prober.AsKReference("t2"), "")))
	} else {
		f.Setup("Create Trigger2 with recorder", triggerresources.Install("t2", brokerName,
			triggerresources.WithSubscriber(prober.AsKReference("t2"), ""),
			delivery.WithDeadLetterSink(prober.AsKReference("t2dlq"), "")))
	}

	// To test, we will have to compute what the expected delivery spec each recorder.
	return prober
}
