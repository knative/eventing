/*
Copyright 2022 The Knative Authors

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

package new_trigger_filters

import (
	"fmt"

	. "github.com/cloudevents/sdk-go/v2/test"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	. "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"github.com/cloudevents/sdk-go/v2/event"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

// FiltersFeatureSet creates a feature set for testing the broker implementation of the new trigger filters experimental feature
// (aka Cloud Events Subscriptions API filters). It requires a created and ready Broker resource with brokerName.
//
// The feature set tests four filter dialects: exact, prefix, suffix and cesql (aka CloudEvents SQL).
func FiltersFeatureSet(brokerName string) *feature.FeatureSet {
	matchedEvent := FullEvent()
	unmatchedEvent := MinEvent()
	unmatchedEvent.SetType("org.wrong.type")
	unmatchedEvent.SetSource("org.wrong.source")

	features := make([]*feature.Feature, 0, 8)
	tests := map[string]struct {
		filters string
		step    feature.StepFn
	}{
		"Exact filter": {
			filters: fmt.Sprintf(snippetFor("exact"), matchedEvent.Type(), matchedEvent.Source()),
		},
		"Prefix filter": {
			filters: fmt.Sprintf(snippetFor("prefix"), matchedEvent.Type()[:4], matchedEvent.Source()[:4]),
		},
		"Suffix filter": {
			filters: fmt.Sprintf(snippetFor("suffix"), matchedEvent.Type()[5:], matchedEvent.Source()[5:]),
		},
		"CloudEvents SQL filter": {
			filters: fmt.Sprintf(`- cesql: "type = '%s' AND source = '%s'" `, matchedEvent.Type(), matchedEvent.Source()),
		},
	}

	for name, fs := range tests {
		matchedSender := feature.MakeRandomK8sName("sender")
		unmatchedSender := feature.MakeRandomK8sName("sender")
		subscriber := feature.MakeRandomK8sName("subscriber")
		triggerName := feature.MakeRandomK8sName("viaTrigger")

		f := feature.NewFeatureNamed(name)

		f.Setup("Install trigger subscriber", eventshub.Install(subscriber, eventshub.StartReceiver))

		// Set the Trigger subscriber.
		cfg := []manifest.CfgFn{
			trigger.WithSubscriber(service.AsKReference(subscriber), ""),
			WithNewFilters(fs.filters),
		}

		f.Setup("Install trigger", trigger.Install(triggerName, brokerName, cfg...))
		f.Setup("Wait for trigger to become ready", trigger.IsReady(triggerName))
		f.Setup("Broker is addressable", k8s.IsAddressable(broker.GVR(), brokerName))

		f.Requirement("Install matched event sender", eventshub.Install(matchedSender,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputEvent(matchedEvent)),
		)

		f.Requirement("Install unmatched event sender", eventshub.Install(unmatchedSender,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputEvent(unmatchedEvent)),
		)

		f.Alpha("Triggers with new filters").
			Must("must deliver matched events", OnStore(subscriber).MatchEvent(HasId(matchedEvent.ID())).AtLeast(1)).
			MustNot("must not deliver unmatched events", OnStore(subscriber).MatchEvent(HasId(unmatchedEvent.ID())).Not())
		features = append(features, f)
	}

	return &feature.FeatureSet{
		Name:     "New trigger filters",
		Features: features,
	}
}

func snippetFor(key string) string {
	return fmt.Sprintf(`
    - %s:
        type: %%s
        source: %%s`, key)
}

// WithNewFilters adds a filter config to a Trigger spec using the new filters API.
func WithNewFilters(filters string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["filters"] = filters
	}
}

type CloudEventsContext struct {
	eventType            string
	eventSource          string
	eventSubject         string
	eventID              string
	eventDataSchema      string
	eventDataContentType string
	shouldDeliver        bool
}

func AnyFilterFeature(brokerName string) *feature.Feature {
	f := feature.NewFeature()

	eventContexts := []CloudEventsContext{
		{
			eventType:     "exact.event.type",
			shouldDeliver: true,
		},
		{
			eventType:     "prefix.event.type",
			shouldDeliver: true,
		},
		{
			eventType:     "event.type.suffix",
			shouldDeliver: true,
		},
		{
			eventType:     "not.type.event",
			shouldDeliver: true,
		},
		{
			eventType:     "cesql.event.type",
			shouldDeliver: true,
		},
		{
			eventType:     "not.event.type",
			shouldDeliver: false,
		},
	}

	filters := []eventingv1.SubscriptionsAPIFilter{
		{
			Any: []eventingv1.SubscriptionsAPIFilter{
				{
					Exact: map[string]string{
						"type": "exact.event.type",
					},
				},
				{
					Prefix: map[string]string{
						"type": "prefix",
					},
				},
				{
					Suffix: map[string]string{
						"type": "suffix",
					},
				},
				{
					Not: &eventingv1.SubscriptionsAPIFilter{
						CESQL: "type LIKE %event.type%",
					},
				},
				{
					CESQL: "type = 'cesql.event.type'",
				},
			},
		},
	}

	f = newEventFilterFeature(eventContexts, filters, f, brokerName)

	return f
}

func newEventFilterFeature(eventContexts []CloudEventsContext, filters []eventingv1.SubscriptionsAPIFilter, f *feature.Feature, brokerName string) *feature.Feature {
	subscriberName := feature.MakeRandomK8sName("subscriber")
	triggerName := feature.MakeRandomK8sName("trigger")

	f.Setup("Install trigger subscriber", eventshub.Install(subscriberName, eventshub.StartReceiver))

	cfg := []manifest.CfgFn{
		trigger.WithSubscriber(service.AsKReference(subscriberName), ""),
		trigger.WithNewFilters(filters),
	}

	f.Setup("Install trigger", trigger.Install(triggerName, brokerName, cfg...))
	f.Setup("Wait for trigger to become ready", trigger.IsReady(triggerName))
	f.Setup("Broker is addressable", k8s.IsAddressable(broker.GVR(), brokerName))

	asserter := f.Alpha("New filters")

	for _, eventCtx := range eventContexts {
		event := newEventFromEventContext(eventCtx)
		eventSender := feature.MakeRandomK8sName("sender")

		f.Requirement(fmt.Sprintf("Install event sender %s", eventSender), eventshub.Install(eventSender,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputEvent(event),
		))

		if eventCtx.shouldDeliver {
			asserter.Must("must deliver matched event", OnStore(subscriberName).MatchEvent(HasId(event.ID())).AtLeast(1))
		} else {
			asserter.MustNot("must not deliver unmatched event", OnStore(subscriberName).MatchEvent(HasId(event.ID())).Not())
		}
	}

	return f
}

func newEventFromEventContext(eventCtx CloudEventsContext) event.Event {
	event := MinEvent()
	// Ensure that each event has a unique ID
	event.SetID(feature.MakeRandomK8sName("event"))
	if eventCtx.eventType != "" {
		event.SetType(eventCtx.eventType)
	}
	if eventCtx.eventSource != "" {
		event.SetSource(eventCtx.eventSource)
	}
	if eventCtx.eventSubject != "" {
		event.SetSubject(eventCtx.eventSubject)
	}
	if eventCtx.eventID != "" {
		event.SetID(eventCtx.eventID)
	}
	if eventCtx.eventDataSchema != "" {
		event.SetDataSchema(eventCtx.eventDataSchema)
	}
	if eventCtx.eventDataContentType != "" {
		event.SetDataContentType(eventCtx.eventDataContentType)
	}
	return event
}
