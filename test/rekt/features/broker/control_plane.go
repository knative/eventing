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

	conformanceevent "github.com/cloudevents/conformance/pkg/event"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
	"knative.dev/reconciler-test/resources/svc"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/features/knconf"
	triggerfeatures "knative.dev/eventing/test/rekt/features/trigger"
	"knative.dev/eventing/test/rekt/resources/broker"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	triggerresources "knative.dev/eventing/test/rekt/resources/trigger"
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
		},
	}

	// Add each feature of event routing and Delivery tests as a new feature
	addControlPlaneEventRouting(fs)
	addControlPlaneDelivery(fs)
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
	bName := feature.MakeRandomK8sName("broker")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("Set Broker Name", setBrokerName(bName))

	f.Setup("install a service", svc.Install(sink, "app", "rekt"))
	brokerOpts := append(brokerresources.WithEnvConfig(), delivery.WithDeadLetterSink(svc.AsKReference(sink), ""))
	f.Setup("update broker", broker.Install(bName, brokerOpts...))
	f.Setup("broker goes ready", broker.IsReady(bName))

	f.Stable("Conformance").
		Should("Broker objects SHOULD include a Ready condition in their status",
			knconf.KResourceHasReadyInConditions(brokerresources.GVR(), brokerName)).
		Should("The Broker SHOULD indicate Ready=True when its ingress is available to receive events.",
			readyBrokerHasIngressAvailable).
		Should("While a Broker is Ready, it SHOULD be a valid Addressable and its `status.address.url` field SHOULD indicate the address of its ingress.",
			readyBrokerIsAddressable).
		Should("The class of a Broker object SHOULD be immutable.",
			brokerClassIsImmutable).
		Should("Set the Broker status.deadLetterSinkURI if there is a valid spec.delivery.deadLetterSink defined",
			BrokerStatusDLSURISet)
	return f
}

func ControlPlaneTrigger_GivenBroker(brokerName string) *feature.Feature {
	f := feature.NewFeatureNamed("Trigger, Given Broker")
	f.Setup("Set Broker Name", setBrokerName(brokerName))

	subscriberName := feature.MakeRandomK8sName("sub")
	f.Setup("Install Subscriber", svc.Install(subscriberName, "bad", "svc"))

	triggerName := feature.MakeRandomK8sName("trigger")
	f.Setup("Create a Trigger", triggerresources.Install(triggerName, brokerName,
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", triggerfeatures.SetTriggerName(triggerName))

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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", triggerfeatures.SetTriggerName(triggerName))

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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", triggerfeatures.SetTriggerName(triggerName))

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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
		triggerresources.WithFilter(filters),
	))

	f.Setup("Set Trigger Name", triggerfeatures.SetTriggerName(triggerName))

	f.Stable("Conformance").
		Must("The attributes filter specifying a list of key-value pairs MUST be supported by Trigger.",
			// Compare the passed filters with what is found on the control plane.
			func(ctx context.Context, t feature.T) {
				trigger := triggerfeatures.GetTrigger(ctx, t)
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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
	))

	f.Setup("Set Trigger Name", triggerfeatures.SetTriggerName(triggerName))

	asserter := f.Stable("Conformance - Negatives - The attributes filter specifying a list of key-value pairs MUST be supported by Trigger.")

	for key, value := range filters {
		k := key
		v := value
		asserter.Must("Reject invalid filter - "+k+" - "+v,
			// Compare the passed filters with what is found on the control plane.
			func(ctx context.Context, t feature.T) {
				trigger := triggerfeatures.GetTrigger(ctx, t)

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

func addControlPlaneDelivery(fs *feature.FeatureSet) {
	for i, tt := range []struct {
		name     string
		brokerDS *v1.DeliverySpec
		// Trigger 1 Delivery spec
		t1DS *v1.DeliverySpec
		// How many events to fail before succeeding
		t1FailCount uint
		// Trigger 2 Delivery spec
		t2DS *v1.DeliverySpec
		// How many events to fail before succeeding
		t2FailCount uint
	}{{
		name: "When `BrokerSpec.Delivery` and `TriggerSpec.Delivery` are both not configured, no delivery spec SHOULD be used.",
	}, {
		name: "When `BrokerSpec.Delivery` is configured, but not the specific `TriggerSpec.Delivery`, then the `BrokerSpec.Delivery` SHOULD be used. (Retry)",
		brokerDS: &v1.DeliverySpec{
			DeadLetterSink: new(duckv1.Destination),
			Retry:          ptr.Int32(3),
		},
		t1FailCount: 3, // Should get event.
		t2FailCount: 4, // Should end up in DLQ.
	}, {
		name: "When `TriggerSpec.Delivery` is configured, then `TriggerSpec.Delivery` SHOULD be used. (Retry)",
		brokerDS: &v1.DeliverySpec{ // Disable delivery spec defaulting
			Retry: ptr.Int32(0),
		},
		t1DS: &v1.DeliverySpec{
			DeadLetterSink: new(duckv1.Destination),
			Retry:          ptr.Int32(3),
		},
		t2DS: &v1.DeliverySpec{
			Retry: ptr.Int32(1),
		},
		t1FailCount: 3, // Should get event.
		t2FailCount: 2, // Should be dropped.
	}, {
		name: "When both `BrokerSpec.Delivery` and `TriggerSpec.Delivery` is configured, then `TriggerSpec.Delivery` SHOULD be used. (Retry)",
		brokerDS: &v1.DeliverySpec{
			DeadLetterSink: new(duckv1.Destination),
			Retry:          ptr.Int32(1),
		},
		t1DS: &v1.DeliverySpec{
			DeadLetterSink: new(duckv1.Destination),
			Retry:          ptr.Int32(3),
		},
		t1FailCount: 3, // Should get event.
		t2FailCount: 2, // Should end up in DLQ.
	}, {
		name: "When both `BrokerSpec.Delivery` and `TriggerSpec.Delivery` is configured, then `TriggerSpec.Delivery` SHOULD be used. (Retry+DLQ)",
		brokerDS: &v1.DeliverySpec{
			DeadLetterSink: new(duckv1.Destination),
			Retry:          ptr.Int32(1),
		},
		t1DS: &v1.DeliverySpec{
			DeadLetterSink: new(duckv1.Destination),
			Retry:          ptr.Int32(3),
		},
		t1FailCount: 4, // Should end up in Trigger DLQ.
		t2FailCount: 2, // Should end up in Broker DLQ.
	}} {
		// TODO: Each of these creates quite a few resources. We need to figure out a way
		// to delete the resources for each Feature once the test completes. Today it's
		// not easy (if at all possible) to do this, since Environment contains the References
		// to created resources, but it's not granular enough.
		brokerName := fmt.Sprintf("dlq-test-%d", i)
		f := feature.NewFeatureNamed(fmt.Sprintf("Delivery Spec - %s", brokerName))
		cfg := []triggerCfg{{
			delivery:  tt.t1DS,
			failCount: tt.t1FailCount,
		}, {
			delivery:  tt.t2DS,
			failCount: tt.t2FailCount,
		}}
		prober := createBrokerTriggerTopology(f, brokerName, tt.brokerDS, cfg)

		// Send an event into the matrix and hope for the best
		prober.SenderFullEvents(1)
		f.Setup("install source", prober.SenderInstall("source"))
		f.Requirement("sender is finished", prober.SenderDone("source"))

		// All events have been sent, time to look at the specs and confirm we got them.
		expectedEvents := createExpectedEventPatterns(tt.brokerDS, cfg)

		f.Requirement("wait until done", func(ctx context.Context, t feature.T) {
			interval, timeout := environment.PollTimingsFromContext(ctx)
			err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				gtg := true
				for prefix, want := range expectedEvents {
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
		f.Stable("Conformance").Should(tt.name, knconf.AssertEventPatterns(prober, expectedEvents))
		f.Teardown("Delete feature resources", f.DeleteResources)
		fs.Features = append(fs.Features, *f)
	}
}

func addControlPlaneEventRouting(fs *feature.FeatureSet) {

	fullEvent := cetest.FullEvent()
	replyEvent := cetest.FullEvent()
	replyEvent.SetType("com.example.ReplyEvent")

	for i, tt := range []struct {
		name     string
		config   []triggerCfg
		inEvents []conformanceevent.Event
	}{{
		name:     "One trigger, no filter, gets event",
		config:   []triggerCfg{{}},
		inEvents: []conformanceevent.Event{knconf.EventToEvent(&fullEvent)},
	}, {
		name: "One trigger, with filter, does not get event",
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "mytype",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "notmytype",
				},
			},
		},
	}, {
		name: "One trigger, with filter, gets the event",
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{knconf.EventToEvent(&fullEvent)},
	}, {
		// name: "Two triggers, with filter, both get the event",
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
					},
				},
			}, {
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{knconf.EventToEvent(&fullEvent)},
	}, {
		name: "Two triggers, with filter, only matching one gets the event",
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "notmytype",
					},
				},
			}, {
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{knconf.EventToEvent(&fullEvent)},
	}, {
		name: "Two triggers, with filter, first one matches incoming event, creates reply, which matches the second one",
		config: []triggerCfg{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
					},
				},
				reply: func() *conformanceevent.Event {
					reply := knconf.EventToEvent(&replyEvent)
					reply.Attributes.DataContentType = "application/json" // EventsHub defaults all data to this.
					return &reply
				}(),
			}, {
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.ReplyEvent",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{knconf.EventToEvent(&fullEvent)},
	}, {
		name:     "Two triggers, with no filters, both get the event",
		config:   []triggerCfg{{}, {}},
		inEvents: []conformanceevent.Event{knconf.EventToEvent(&fullEvent)},
	}} {
		brokerName := fmt.Sprintf("routing-test-%d", i)
		f := feature.NewFeatureNamed(fmt.Sprintf("Event Routing Spec - %s", brokerName))
		f.Setup("Set Broker Name", setBrokerName(brokerName))
		prober := createBrokerTriggerTopology(f, brokerName, nil, tt.config)

		// Send an event into the matrix and hope for the best
		// TODO: We need to do some work to get the event types into the Prober.
		// All the events generated are currently hardcoded into the com.example.FullEvent
		// so once prober supports more configuration, wire it up here.
		prober.SenderFullEvents(1)
		f.Setup("install source", prober.SenderInstall("source"))
		f.Requirement("sender is finished", prober.SenderDone("source"))

		// All events have been sent, time to look at the specs and confirm we got them.
		expectedEvents := createExpectedEventRoutingMap(tt.config, tt.inEvents)

		f.Requirement("wait until done", func(ctx context.Context, t feature.T) {
			interval, timeout := environment.PollTimingsFromContext(ctx)
			err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				gtg := true
				for prefix, want := range expectedEvents {
					events := prober.ReceivedOrRejectedBy(ctx, prefix)
					if len(events) != len(want) {
						gtg = false
					}
				}
				return gtg, nil
			})
			if err != nil {
				t.Failed()
			}
		})

		f.Stable("Conformance").Should(tt.name, assertExpectedRoutedEvents(prober, expectedEvents))
		f.Teardown("Delete feature resources", f.DeleteResources)
		fs.Features = append(fs.Features, *f)
	}
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
	BrokerNameKey = "brokerName"
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

func readyBrokerHasIngressAvailable(ctx context.Context, t feature.T) {
	// TODO: I am not sure how to test this from the outside.
}

func readyBrokerIsAddressable(ctx context.Context, t feature.T) {
	broker := getBroker(ctx, t)

	if broker.IsReady() {
		if broker.Status.Address.URL == nil {
			t.Errorf("broker is not addressable")
		}
		// Success!
	} else {
		t.Errorf("broker was not ready, reason: %s", broker.Status.GetTopLevelCondition().Reason)
	}
}

func BrokerStatusDLSURISet(ctx context.Context, t feature.T) {
	broker := getBroker(ctx, t)

	if broker.IsReady() {
		if broker.Status.DeadLetterSinkURI == nil {
			t.Errorf("broker DLS not resolved but resource reported ready")
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
		trigger = triggerfeatures.GetTrigger(ctx, t)
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
	trigger := triggerfeatures.GetTrigger(ctx, t)
	_ = trigger
	// TODO: I am not sure how to test this from the outside.
}

func readyTriggerHasSubscriberURI(ctx context.Context, t feature.T) {
	trigger := triggerfeatures.GetTrigger(ctx, t)
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
	trigger := triggerfeatures.GetTrigger(ctx, t)
	if trigger.Spec.Broker == "" {
		t.Error("broker is empty")
	}
	if strings.Contains(trigger.Spec.Broker, ",") {
		t.Errorf("more than one broker specified: %q", trigger.Spec.Broker)
	}
}

func triggerSpecBrokerIsImmutable(ctx context.Context, t feature.T) {
	trigger := triggerfeatures.GetTrigger(ctx, t)

	// Update spec.broker
	trigger.Spec.Broker = "Rekt.BrokerImmutable"

	if _, err := Client(ctx).Triggers.Update(ctx, trigger, metav1.UpdateOptions{}); err != nil {
		// Success!
		t.Log("Trigger spec.broker is immutable")
	} else {
		t.Errorf("Trigger spec.broker is mutable")
	}
}
