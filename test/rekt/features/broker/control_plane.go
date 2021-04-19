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
	"sort"
	"strings"

	conformanceevent "github.com/cloudevents/conformance/pkg/event"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/features/knconf"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	triggerresources "knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/state"
	"knative.dev/reconciler-test/resources/svc"
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
			*ControlPlaneDelivery(),
			*ControlPlaneEventRouting(),
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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
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
		triggerresources.WithSubscriber(svc.AsKReference(subscriberName), ""),
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

func ControlPlaneDelivery() *feature.Feature {
	f := feature.NewFeatureNamed("Delivery Spec")

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
		t1DS: &v1.DeliverySpec{
			DeadLetterSink: new(duckv1.Destination),
			Retry:          ptr.Int32(3),
		},
		t1FailCount: 3, // Should get event.
		t2FailCount: 1, // Should be dropped.
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
	}} {
		brokerName := fmt.Sprintf("dlq-test-%d", i)
		prober := createBrokerTriggerDeliveryTopology(f, brokerName, tt.brokerDS, tt.t1DS, tt.t2DS, tt.t1FailCount, tt.t2FailCount)

		// Send an event into the matrix and hope for the best
		prober.SenderFullEvents(1)
		f.Setup("install source", prober.SenderInstall("source"))
		f.Requirement("sender is finished", prober.SenderDone("source"))

		// All events have been sent, time to look at the specs and confirm we got them.
		expectedEvents := createExpectedEventMap(tt.brokerDS, tt.t1DS, tt.t2DS, tt.t1FailCount, tt.t2FailCount)

		f.Requirement("wait until done", func(ctx context.Context, t feature.T) {
			interval, timeout := environment.PollTimingsFromContext(ctx)
			err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				gtg := true
				for prefix, want := range expectedEvents {
					events := prober.ReceivedOrRejectedBy(ctx, prefix)
					if len(events) != len(want.eventSuccess) {
						gtg = false
					}
				}
				return gtg, nil
			})
			if err != nil {
				t.Failed()
			}
		})

		f.Stable("Conformance").Should(tt.name, assertExpectedEvents(prober, expectedEvents))
	}

	return f
}

func ControlPlaneEventRouting() *feature.Feature {
	f := feature.NewFeatureNamed("Event Routing Spec")

	for i, tt := range []struct {
		name     string
		config   []triggerTestConfig
		inEvents []conformanceevent.Event
	}{{
		name:   "One trigger, no filter, gets event",
		config: []triggerTestConfig{{}},
		inEvents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "com.example.FullEvent",
				},
			},
		},
	}, {
		name: "One trigger, with filter, does not get event",
		config: []triggerTestConfig{
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
		config: []triggerTestConfig{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
						//						"type": "mytype",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "mytype",
				},
			},
		},
	}, {
		// name: "Two triggers, with filter, both get the event",
		config: []triggerTestConfig{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
						//						"type": "mytype",
					},
				},
			},
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
						//						"type": "mytype",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					//					Type: "mytype",
					Type: "com.example.FullEvent",
				},
			},
		},
	}, {
		name: "Two triggers, with filter, only matching one gets the event",
		config: []triggerTestConfig{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "notmytype",
					},
				},
			},
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
						//						"type": "mytype",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					//					Type: "mytype",
					Type: "com.example.FullEvent",
				},
			},
		},
	}, {
		name: "Two triggers, with filter, first one matches incoming event, creates reply, which matches the second one",
		config: []triggerTestConfig{
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.FullEvent",
					},
				},
				reply: &conformanceevent.Event{
					Attributes: conformanceevent.ContextAttributes{
						Type: "com.example.ReplyEvent",
					},
				},
			},
			{
				filter: &eventingv1.TriggerFilter{
					Attributes: eventingv1.TriggerFilterAttributes{
						"type": "com.example.ReplyEvent",
					},
				},
			},
		},
		inEvents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					Type: "com.example.FullEvent",
				},
			},
		},
	}, {
		name:   "Two triggers, with no filters, both get the event",
		config: []triggerTestConfig{{}, {}},
		inEvents: []conformanceevent.Event{
			{
				Attributes: conformanceevent.ContextAttributes{
					//					"type": "com.example.FullEvent",
					Type: "com.example.FullEvent",
				},
			},
		},
	}} {
		brokerName := fmt.Sprintf("routing-test-%d", i)
		f.Setup("Set Broker Name", setBrokerName(brokerName))
		prober := createBrokerTriggerEventRoutingTopology(f, brokerName, tt.config)

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
	}

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

//
// createBrokerTriggerDeliveryTopology creates a topology that allows us to test the various
// delivery configurations.
//
// source ---> [broker (brokerDS)] --+--[trigger1 (t1ds)]--> "t1"
//                         |         |              |
//                         |         |              +--> "t1dlq" (optional)
//                         |         |
//                         |         +-[trigger2 (t2ds)]--> "t2"
//                         |                       |
//                         |                       +--> "t2dlq" (optional)
//                         |
//                         +--[DLQ]--> "dlq" (optional)
//
func createBrokerTriggerDeliveryTopology(f *feature.Feature, brokerName string, brokerDS, t1DS, t2DS *v1.DeliverySpec, t1FailCount, t2FailCount uint) *eventshub.EventProber {
	prober := eventshub.NewProber()

	// TODO: Optimize these to only install things required. For example, if there's no t2 dlq, no point creating a prober for it.
	f.Setup("install recorder for t1", prober.ReceiverInstall("t1", eventshub.DropFirstN(t1FailCount)))
	f.Setup("install recorder for t1dlq", prober.ReceiverInstall("t1dlq"))
	f.Setup("install recorder for t2", prober.ReceiverInstall("t2", eventshub.DropFirstN(t2FailCount)))
	f.Setup("install recorder for t2dlq", prober.ReceiverInstall("t2dlq"))
	f.Setup("install recorder for broker dlq", prober.ReceiverInstall("brokerdlq"))

	brokerOpts := brokerresources.WithEnvConfig()

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
	t1Opts := []manifest.CfgFn{triggerresources.WithSubscriber(prober.AsKReference("t1"), "")}

	if t1DS != nil {
		if t1DS.DeadLetterSink != nil {
			t1Opts = append(t1Opts, delivery.WithDeadLetterSink(prober.AsKReference("t1dlq"), ""))
		}
		if t1DS.Retry != nil {
			t1Opts = append(t1Opts, delivery.WithRetry(*t1DS.Retry, t1DS.BackoffPolicy, t1DS.BackoffDelay))
		}
	}
	f.Setup("Create Trigger1 with recorder", triggerresources.Install(feature.MakeRandomK8sName("t1"), brokerName, t1Opts...))

	t2Opts := []manifest.CfgFn{triggerresources.WithSubscriber(prober.AsKReference("t2"), "")}
	if t2DS != nil {
		if t2DS.DeadLetterSink != nil {
			t2Opts = append(t2Opts, delivery.WithDeadLetterSink(prober.AsKReference("t2dlq"), ""))
		}
		if t2DS.Retry != nil {
			t2Opts = append(t2Opts, delivery.WithRetry(*t2DS.Retry, t2DS.BackoffPolicy, t2DS.BackoffDelay))
		}
	}
	f.Setup("Create Trigger2 with recorder", triggerresources.Install(feature.MakeRandomK8sName("t2"), brokerName, t2Opts...))
	return prober
}

type expectedEvents struct {
	eventSuccess []bool // events and their outcomes (succeeded or failed) in order received by the Receiver
	// What is the minimum time between events above. If there's only one entry, it's irrelevant and will be useless. If there's, say
	// two entries, the second entry is the time between the first and second event. And yeah, there should be 2 events in the above array.
	eventInterval []uint
}

func retryCount(r *int32) uint {
	if r == nil {
		return 0
	}
	return uint(*r)
}

// createExpectedEventMap creates a datastructure for a given test topology created by `createBrokerTriggerDeliveryTopology` function.
// Things we know from the DeliverySpecs passed in are where failed events from both t1 and t2 should land in.
// We also know how many events (incoming as well as how many failures the trigger subscriber is supposed to see).
// Note there are lot of baked assumptions and very tight coupling between this and `createBrokerTriggerDeliveryTopology` function.
func createExpectedEventMap(brokerDS, t1DS, t2DS *v1.DeliverySpec, t1FailCount, t2FailCount uint) map[string]expectedEvents {
	// By default, assume that nothing gets anything.
	r := map[string]expectedEvents{
		"t1": {
			eventSuccess:  []bool{},
			eventInterval: []uint{},
		},
		"t2": {
			eventSuccess:  []bool{},
			eventInterval: []uint{},
		},
		"t1dlq": {
			eventSuccess:  []bool{},
			eventInterval: []uint{},
		},
		"t2dlq": {
			eventSuccess:  []bool{},
			eventInterval: []uint{},
		},
		"brokerdlq": {
			eventSuccess:  []bool{},
			eventInterval: []uint{},
		},
	}

	// For now we assume that there is only one incoming event that will then get retried at respective triggers according
	// to their Delivery policy. Also depending on how the Delivery is configured, it may be delivered to triggers DLQ or
	// the Broker DLQ.
	if t1DS != nil && t1DS.DeadLetterSink != nil {
		// There's a dead letter sink specified. Events can end up here if t1FailCount is greater than retry count
		retryCount := retryCount(t1DS.Retry)
		if t1FailCount >= retryCount {
			// Ok, so we should have more failures than retries => one event in the t1dlq
			r["t1dlq"] = expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
			}
		}
	}

	if t2DS != nil && t2DS.DeadLetterSink != nil {
		// There's a dead letter sink specified. Events can end up here if t1FailCount is greater than retry count
		retryCount := retryCount(t2DS.Retry)
		if t2FailCount >= retryCount {
			// Ok, so we should have more failures than retries => one event in the t1dlq
			r["t2dlq"] = expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
			}
		}
	}

	if brokerDS != nil && brokerDS.DeadLetterSink != nil {
		// There's a dead letter sink specified. Events can end up here if t1FailCount or t2FailCount is greater than retry count
		retryCount := retryCount(brokerDS.Retry)
		if t2FailCount >= retryCount || t1FailCount >= retryCount {
			// Ok, so we should have more failures than retries => one event in the t1dlq
			r["brokerdlq"] = expectedEvents{
				eventSuccess:  []bool{true},
				eventInterval: []uint{0},
			}
		}
	}

	// Ok, so that basically hopefully took care of if any of the DLQs should get events.

	// Default is that there are no retries (they will get constructed below if there are), so assume
	// no retries and failure or success based on the t1FailCount
	r["t1"] = expectedEvents{
		eventSuccess:  []bool{t1FailCount == 0},
		eventInterval: []uint{0},
	}

	if t1DS != nil || brokerDS != nil {
		// Check to see which DeliverySpec applies to Trigger
		effectiveT1DS := t1DS
		if t1DS == nil {
			effectiveT1DS = brokerDS
		}
		r["t1"] = helper(retryCount(effectiveT1DS.Retry), t1FailCount, true)
	}
	r["t2"] = expectedEvents{
		eventSuccess:  []bool{t2FailCount == 0},
		eventInterval: []uint{0},
	}
	if t2DS != nil || brokerDS != nil {
		// Check to see which DeliverySpec applies to Trigger
		effectiveT2DS := t2DS
		if t2DS == nil {
			effectiveT2DS = brokerDS
		}
		r["t2"] = helper(retryCount(effectiveT2DS.Retry), t2FailCount, true)
	}
	return r
}

func helper(retry, failures uint, isLinear bool) expectedEvents {
	if retry == 0 {
		return expectedEvents{
			eventSuccess:  []bool{failures == 0},
			eventInterval: []uint{0},
		}

	}
	r := expectedEvents{
		eventSuccess:  make([]bool, 0),
		eventInterval: make([]uint, 0),
	}
	for i := uint(0); i <= retry; i++ {
		if failures == i {
			r.eventSuccess = append(r.eventSuccess, true)
			r.eventInterval = append(r.eventInterval, 0)
			break
		}
		r.eventSuccess = append(r.eventSuccess, false)
		r.eventInterval = append(r.eventInterval, 0)
	}
	return r
}

func assertExpectedEvents(prober *eventshub.EventProber, expected map[string]expectedEvents) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		for prefix, want := range expected {
			got := happened(ctx, prober, prefix)

			t.Logf("Expected Events %s; \nGot: %#v\n Want: %#v", prefix, got, want)

			// Check event acceptance.
			if len(want.eventSuccess) != 0 && len(got.eventSuccess) != 0 {
				if diff := cmp.Diff(want.eventSuccess, got.eventSuccess); diff != "" {
					t.Error("unexpected event acceptance behaviour (-want, +got) =", diff)
				}
			}
			// TODO: Check timing.
			//if len(want.eventInterval) != 0 && len(got.eventInterval) != 0 {
			//	if diff := cmp.Diff(want.eventInterval, got.eventInterval); diff != "" {
			//		t.Error("unexpected event interval behaviour (-want, +got) =", diff)
			//	}
			//}
		}
	}
}

func assertExpectedRoutedEvents(prober *eventshub.EventProber, expected map[string][]conformanceevent.Event) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		for prefix, want := range expected {
			got := happenedFullEvent(ctx, prober, prefix)

			t.Logf("Expected Events %s; \nGot: %#v\n Want: %#v", prefix, got, want)
			if len(want) != len(got) {
				t.Errorf("Wanted %d events, got %d", len(want), len(got))
			}

			// Check event acceptance.
			if len(want) != 0 && len(got) != 0 {
				if diff := cmp.Diff(want, got); diff != "" {
					t.Error("unexpected event routing behaviour (-want, +got) =", diff)
				}
			}
		}
	}
}

// TODO: this function could be moved to the prober directly.
func happened(ctx context.Context, prober *eventshub.EventProber, prefix string) expectedEvents {
	events := prober.ReceivedOrRejectedBy(ctx, prefix)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})
	got := expectedEvents{
		eventSuccess:  make([]bool, 0),
		eventInterval: make([]uint, 0),
	}
	for i, event := range events {
		got.eventSuccess = append(got.eventSuccess, event.Kind == eventshub.EventReceived)
		if i == 0 {
			got.eventInterval = []uint{0}
		} else {
			diff := events[i-1].Time.Unix() - event.Time.Unix()
			got.eventInterval = append(got.eventInterval, uint(diff))
		}
	}
	return got
}

// TODO: this function could be moved to the prober directly.
func happenedFullEvent(ctx context.Context, prober *eventshub.EventProber, prefix string) []conformanceevent.Event {
	events := prober.ReceivedOrRejectedBy(ctx, prefix)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})
	ret := make([]conformanceevent.Event, len(events))
	for i, e := range events {
		// TODO: yeah, like full event please...
		ret[i] = conformanceevent.Event{
			Attributes: conformanceevent.ContextAttributes{
				Type: e.Event.Type(),
			}}
	}
	return ret

}

// triggerTestConfig is used to define each Trigger behaviour used to construct the test topology by
// createBrokerTriggerEventRoutingTopology
type triggerTestConfig struct {
	filter *eventingv1.TriggerFilter
	reply  *conformanceevent.Event
}

//
// createBrokerTriggerEventRoutingTopology creates a topology that allows us to test the various
// trigger filter configurations. For each triggerConfig a Trigger will be created with an optional
// filter as well as a reply that it will generate in response to any event that it receives.
// For each entry in the triggerConfigs ("i"th entry will be ti in the picture below). If there's
// a filter specified trigger is configured with it. If there's a reply, then the "t0" will be configured
// to reply with that event. **REPLY FUNCTIONALITY DOES NOT WORK YET**
// TODO: Fix the reply.
//
// source ---> [broker] --+--[t0 (optional filter)]--> "t0" (optional reply)
//                        |
//                        +--[t1 (optional filter)]--> "t1" (optional reply)
//                        ...
//                        +--[tN (optional filter)]--> "tN" (optional reply)
//
func createBrokerTriggerEventRoutingTopology(f *feature.Feature, brokerName string, triggerConfigs []triggerTestConfig) *eventshub.EventProber {
	prober := eventshub.NewProber()

	// Install the receivers for all the triggers
	for i, config := range triggerConfigs {
		triggerName := fmt.Sprintf("t%d", i)
		var proberOpts []eventshub.EventsHubOption
		if config.reply != nil {
			proberOpts = append(proberOpts, eventshub.ReplyWithTransformedEvent(config.reply.Attributes.Type, config.reply.Attributes.Source, config.reply.Data))
		}
		f.Setup(fmt.Sprintf("install recorder for %s", triggerName), prober.ReceiverInstall(triggerName, proberOpts...))
	}

	brokerOpts := brokerresources.WithEnvConfig()

	f.Setup("Create Broker", brokerresources.Install(brokerName, brokerOpts...))
	f.Setup("Broker is Ready", brokerresources.IsReady(brokerName)) // We want to block until broker is ready to go.

	prober.SetTargetResource(brokerresources.GVR(), brokerName)

	for i, config := range triggerConfigs {
		triggerName := fmt.Sprintf("t%d", i)
		tOpts := []manifest.CfgFn{triggerresources.WithSubscriber(prober.AsKReference(triggerName), "")}
		if config.filter != nil {
			tOpts = append(tOpts, triggerresources.WithFilter(config.filter.Attributes))
		}
		f.Setup(fmt.Sprintf("Create %s with recorder", triggerName), triggerresources.Install(feature.MakeRandomK8sName(triggerName), brokerName, tOpts...))
	}
	return prober
}

// createExpectedEventRoutingMap takes in an array of trigger configurations as well as incoming events and
// constructs a map of where the events should land. Any replies in trigger configurations will be treated
// as matchable events (since they are going to be sent back to the Broker).
// TODO: This function only handles replies generated to incoming events properly and it's fine for our
// tests for now. But if you wanted to test calling filter T0 which would generated EReply which would
// match filter T1 which would generate EReplyTwo, then that won't work.
func createExpectedEventRoutingMap(triggerConfigs []triggerTestConfig, inEvents []conformanceevent.Event) map[string][]conformanceevent.Event {
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
