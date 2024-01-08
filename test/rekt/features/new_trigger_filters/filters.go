/*
Copyright 2023 The Knative Authors

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
	"knative.dev/reconciler-test/pkg/eventshub"
	. "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"github.com/cloudevents/sdk-go/v2/event"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

func createNewFiltersFeature(f *feature.Feature, eventContexts []CloudEventsContext, filters []eventingv1.SubscriptionsAPIFilter, filter eventingv1.TriggerFilter, installBroker InstallBrokerFunc) {
	subscriberName := feature.MakeRandomK8sName("subscriber")
	triggerName := feature.MakeRandomK8sName("trigger")
	brokerName := installBroker(f)

	f.Setup("Install trigger subscriber", eventshub.Install(subscriberName, eventshub.StartReceiver))

	cfg := []manifest.CfgFn{
		trigger.WithSubscriber(service.AsKReference(subscriberName), ""),
		trigger.WithNewFilters(filters),
		trigger.WithFilter(filter.Attributes),
	}

	f.Setup("Install trigger", trigger.Install(triggerName, brokerName, cfg...))
	f.Setup("Wait for trigger to become ready", trigger.IsReady(triggerName))

	asserter := f.Beta("New filters")

	for _, eventCtx := range eventContexts {
		e := newEventFromEventContext(eventCtx)
		eventSender := feature.MakeRandomK8sName("sender")

		f.Requirement(fmt.Sprintf("Install event sender %s", eventSender), eventshub.Install(eventSender,
			eventshub.StartSenderToResource(broker.GVR(), brokerName),
			eventshub.InputEvent(e),
		))

		if eventCtx.shouldDeliver {
			asserter.Must("must deliver matched event", OnStore(subscriberName).MatchEvent(HasId(e.ID())).AtLeast(1))
		} else {
			asserter.MustNot("must not deliver unmatched event", OnStore(subscriberName).MatchEvent(HasId(e.ID())).Not())
		}
	}
}

func newEventFromEventContext(eventCtx CloudEventsContext) event.Event {
	e := MinEvent()
	// Ensure that each event has a unique ID
	e.SetID(feature.MakeRandomK8sName("event"))
	if eventCtx.eventType != "" {
		e.SetType(eventCtx.eventType)
	}
	if eventCtx.eventSource != "" {
		e.SetSource(eventCtx.eventSource)
	}
	if eventCtx.eventSubject != "" {
		e.SetSubject(eventCtx.eventSubject)
	}
	if eventCtx.eventID != "" {
		e.SetID(eventCtx.eventID)
	}
	if eventCtx.eventDataSchema != "" {
		e.SetDataSchema(eventCtx.eventDataSchema)
	}
	if eventCtx.eventDataContentType != "" {
		e.SetDataContentType(eventCtx.eventDataContentType)
	}
	if eventCtx.eventExtensions != nil {
		for k, v := range eventCtx.eventExtensions {
			e.SetExtension(k, v)
		}
	}
	return e
}
