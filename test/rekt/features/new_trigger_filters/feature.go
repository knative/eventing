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
	"context"
	"fmt"

	. "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

type InstallBrokerFunc func(brokerName string) feature.StepFn

type CloudEventsContext struct {
	eventType            string
	eventSource          string
	eventSubject         string
	eventID              string
	eventDataSchema      string
	eventDataContentType string
	eventExtensions      map[string]interface{}
	shouldDeliver        bool
}

// NewFiltersFeatureSet creates a feature set which runs tests for the new trigger filters
// It requires a function which installs a broker implementation into the current feature for testing.
// All new triggers tests should be registered in this feature set so that they can be easily included in other
// broker implementations for testing.
func NewFiltersFeatureSet(installBroker InstallBrokerFunc) *feature.FeatureSet {
	features := SingleDialectFilterFeatures(installBroker)
	features = append(
		features,
		AllFilterFeature(installBroker),
		AnyFilterFeature(installBroker),
		MultipleTriggersAndSinksFeature(installBroker),
		MissingAttributesFeature(installBroker),
		FilterAttributeWithEmptyFiltersFeature(installBroker),
		FiltersOverrideAttributeFilterFeature(installBroker),
		MultipleFiltersFeature(installBroker),
	)
	return &feature.FeatureSet{
		Name:     "New Trigger Filters",
		Features: features,
	}
}

// SingleDialectFilterFeatures creates an array of features which each test a single dialect of the new filters.
// It requires a function which installs a broker implementation into the current feature for testing.
func SingleDialectFilterFeatures(installBroker InstallBrokerFunc) []*feature.Feature {
	matchedEvent := FullEvent()
	unmatchedEvent := MinEvent()
	unmatchedEvent.SetType("org.wrong.type")
	unmatchedEvent.SetSource("org.wrong.source")

	eventContexts := []CloudEventsContext{
		{
			eventType:     matchedEvent.Type(),
			eventSource:   matchedEvent.Source(),
			shouldDeliver: true,
		},
		{
			eventType:     unmatchedEvent.Type(),
			eventSource:   unmatchedEvent.Source(),
			shouldDeliver: false,
		},
	}

	features := make([]*feature.Feature, 0, 8)
	tests := map[string]struct {
		filters []eventingv1.SubscriptionsAPIFilter
	}{
		"Exact filter": {
			filters: []eventingv1.SubscriptionsAPIFilter{
				{
					Exact: map[string]string{
						"type":   matchedEvent.Type(),
						"source": matchedEvent.Source(),
					},
				},
			},
		},
		"Prefix filter": {
			filters: []eventingv1.SubscriptionsAPIFilter{
				{
					Prefix: map[string]string{
						"type":   matchedEvent.Type()[:4],
						"source": matchedEvent.Source()[:4],
					},
				},
			},
		},
		"Suffix filter": {
			filters: []eventingv1.SubscriptionsAPIFilter{
				{
					Suffix: map[string]string{
						"type":   matchedEvent.Type()[5:],
						"source": matchedEvent.Source()[5:],
					},
				},
			},
		},
		"CloudEvents SQL filter": {
			filters: []eventingv1.SubscriptionsAPIFilter{
				{
					CESQL: fmt.Sprintf("type = '%s' AND source = '%s'", matchedEvent.Type(), matchedEvent.Source()),
				},
			},
		},
	}

	for name, fs := range tests {
		f := feature.NewFeatureNamed(name)
		createNewFiltersFeature(f, eventContexts, fs.filters, eventingv1.TriggerFilter{}, installBroker)
		features = append(features, f)
	}

	return features
}

func AnyFilterFeature(installBroker InstallBrokerFunc) *feature.Feature {
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
						CESQL: "type LIKE '%event.type%'",
					},
				},
				{
					CESQL: "type = 'cesql.event.type'",
				},
			},
		},
	}

	createNewFiltersFeature(f, eventContexts, filters, eventingv1.TriggerFilter{}, installBroker)

	return f
}

func AllFilterFeature(installBroker InstallBrokerFunc) *feature.Feature {
	f := feature.NewFeature()

	eventContexts := []CloudEventsContext{
		// This event matches no filters
		{
			eventType:     "not.event.type",
			shouldDeliver: false,
		},
		// This event matches 2 filters: prefix and CESQL.
		{
			eventType:     "exact.prefix.suffix.event",
			shouldDeliver: false, // This should not get delivered as not all filters match.
		},
		// This event matches 3 filters: CESQL, Prefix, and Suffix.
		{
			eventType:     "exact.prefix.suffix.event.suffix.event.type",
			shouldDeliver: false, // This should not get delivered as not all filters match.
		},
		// This event will match all 4 filters.
		{
			eventType:     "exact.prefix.suffix.event.type",
			shouldDeliver: true,
		},
	}

	filters := []eventingv1.SubscriptionsAPIFilter{
		{
			All: []eventingv1.SubscriptionsAPIFilter{
				{
					Exact: map[string]string{
						"type": "exact.prefix.suffix.event.type",
					},
				},
				{
					Prefix: map[string]string{
						"type": "exact.prefix",
					},
				},
				{
					Suffix: map[string]string{
						"type": "suffix.event.type",
					},
				},
				{
					CESQL: "type LIKE 'exact.prefix.suffix%'",
				},
			},
		},
	}

	createNewFiltersFeature(f, eventContexts, filters, eventingv1.TriggerFilter{}, installBroker)

	return f
}

func MultipleTriggersAndSinksFeature(installBroker InstallBrokerFunc) *feature.Feature {
	f := feature.NewFeature()

	eventContextsFirstSink := []CloudEventsContext{
		{
			eventType:     "type1",
			shouldDeliver: true,
		},
		{
			eventType:     "both.should.match",
			shouldDeliver: true,
		},
		{
			eventType:     "type2",
			shouldDeliver: false,
		},
		{
			eventType:     "type3",
			shouldDeliver: false,
		},
	}

	filtersFirstTrigger := []eventingv1.SubscriptionsAPIFilter{
		{
			Any: []eventingv1.SubscriptionsAPIFilter{
				{
					Exact: map[string]string{
						"type": "type1",
					},
				},
				{
					Prefix: map[string]string{
						"type": "both",
					},
				},
			},
		},
	}

	eventContextsSecondSink := []CloudEventsContext{
		{
			eventType:     "type1",
			shouldDeliver: false,
		},
		{
			eventType:     "both.should.match",
			shouldDeliver: true,
		},
		{
			eventType:     "type2",
			shouldDeliver: true,
		},
		{
			eventType:     "type3",
			shouldDeliver: false,
		},
	}

	filtersSecondTrigger := []eventingv1.SubscriptionsAPIFilter{
		{
			Any: []eventingv1.SubscriptionsAPIFilter{
				{
					Exact: map[string]string{
						"type": "type2",
					},
				},
				{
					Prefix: map[string]string{
						"type": "both",
					},
				},
			},
		},
	}

	subscriberName1 := feature.MakeRandomK8sName("subscriber1")
	subscriberName2 := feature.MakeRandomK8sName("subscriber2")
	triggerName1 := feature.MakeRandomK8sName("trigger1")
	triggerName2 := feature.MakeRandomK8sName("trigger2")
	brokerName := feature.MakeRandomK8sName("broker")

	f.Setup("Install Broker, Sinks, Triggers", func(ctx context.Context, t feature.T) {
		installBroker(brokerName)(ctx, t)

		eventshub.Install(subscriberName1, eventshub.StartReceiver)(ctx, t)
		eventshub.Install(subscriberName2, eventshub.StartReceiver)(ctx, t)

		triggerCfg1 := []manifest.CfgFn{
			trigger.WithSubscriber(service.AsKReference(subscriberName1), ""),
			trigger.WithNewFilters(filtersFirstTrigger),
			trigger.WithFilter(eventingv1.TriggerFilter{}.Attributes),
		}
		trigger.Install(triggerName1, brokerName, triggerCfg1...)(ctx, t)

		triggerCfg2 := []manifest.CfgFn{
			trigger.WithSubscriber(service.AsKReference(subscriberName2), ""),
			trigger.WithNewFilters(filtersSecondTrigger),
			trigger.WithFilter(eventingv1.TriggerFilter{}.Attributes),
		}
		trigger.Install(triggerName2, brokerName, triggerCfg2...)(ctx, t)

		broker.IsReady(brokerName)(ctx, t)
		broker.IsAddressable(brokerName)(ctx, t)
		trigger.IsReady(triggerName1)(ctx, t)
		trigger.IsReady(triggerName2)(ctx, t)
	})

	assertDelivery(f, brokerName, subscriberName1, eventContextsFirstSink)
	assertDelivery(f, brokerName, subscriberName2, eventContextsSecondSink)

	return f
}

func MissingAttributesFeature(installBroker InstallBrokerFunc) *feature.Feature {
	f := feature.NewFeature()

	eventContexts := []CloudEventsContext{
		// this event will have no extension, so the filters should all fail
		{
			shouldDeliver: false,
		},
		// This event has the extension, so the filters shold all pass
		{
			eventExtensions: map[string]interface{}{
				"extensionattribute": "extensionvalue",
			},
			shouldDeliver: true,
		},
	}

	filters := []eventingv1.SubscriptionsAPIFilter{
		{
			All: []eventingv1.SubscriptionsAPIFilter{
				{
					Exact: map[string]string{
						"extensionattribute": "extensionvalue",
					},
				},
				{
					Prefix: map[string]string{
						"extensionattribute": "extension",
					},
				},
				{
					Suffix: map[string]string{
						"extensionattribute": "value",
					},
				},
				{
					CESQL: "extensionattribute LIKE '%value'",
				},
			},
		},
	}

	createNewFiltersFeature(f, eventContexts, filters, eventingv1.TriggerFilter{}, installBroker)

	return f
}

// Empty SubscriptionAPI Filters do not override Filter Attributes
func FilterAttributeWithEmptyFiltersFeature(installBroker InstallBrokerFunc) *feature.Feature {
	f := feature.NewFeature()

	eventContexts := []CloudEventsContext{
		// This event matches the filter attribute
		{
			eventType:     "filter.attribute.event.type",
			shouldDeliver: true,
		},
		// This event matches no filter attribute
		{
			eventType:     "some.other.type",
			shouldDeliver: false,
		},
	}

	filter := eventingv1.TriggerFilter{
		Attributes: map[string]string{
			"type": "filter.attribute.event.type",
		},
	}

	createNewFiltersFeature(f, eventContexts, []eventingv1.SubscriptionsAPIFilter{}, filter, installBroker)

	return f
}

// SubscriptionAPI filters override filter attributes
func FiltersOverrideAttributeFilterFeature(installBroker InstallBrokerFunc) *feature.Feature {
	f := feature.NewFeature()

	eventContexts := []CloudEventsContext{
		// This event matches the filters in subscriptionAPI and does not match filter attribute.
		{
			eventType:     "subscriptionapi.filters.override.event.type",
			shouldDeliver: true,
		},
		// This event matches the filter attribute and does not match filters in subscriptionAPI.
		{
			eventType:     "filter.check.event.type",
			shouldDeliver: false,
		},
	}

	filters := []eventingv1.SubscriptionsAPIFilter{
		{
			Exact: map[string]string{
				"type": "subscriptionapi.filters.override.event.type",
			},
		},
	}

	filter := eventingv1.TriggerFilter{
		Attributes: map[string]string{
			"type": "filter.check.event.type",
		},
	}

	createNewFiltersFeature(f, eventContexts, filters, filter, installBroker)

	return f
}

// Multiple filters are specified without All or Any option.
func MultipleFiltersFeature(installBroker InstallBrokerFunc) *feature.Feature {
	f := feature.NewFeature()

	eventContexts := []CloudEventsContext{
		// This event matches no filters.
		{
			eventType:     "not.event.type",
			shouldDeliver: false,
		},
		// This event matches 2 filters: prefix and CESQL.
		{
			eventType:     "exact.prefix.suffix.event",
			shouldDeliver: false, // This should not get delivered as not all filters match.
		},
		// This event matches 3 filters: CESQL, Prefix, and Suffix.
		{
			eventType:     "exact.prefix.suffix.event.suffix.event.type",
			shouldDeliver: false, // This should not get delivered as not all filters match.
		},
		// This event will match all 4 filters.
		{
			eventType:     "exact.prefix.suffix.event.type",
			shouldDeliver: true,
		},
	}

	filters := []eventingv1.SubscriptionsAPIFilter{
		{
			Exact: map[string]string{
				"type": "exact.prefix.suffix.event.type",
			},
		},
		{
			Prefix: map[string]string{
				"type": "exact.prefix",
			},
		},
		{
			Suffix: map[string]string{
				"type": "suffix.event.type",
			},
		},
		{
			CESQL: "type LIKE 'exact.prefix.suffix%'",
		},
	}

	createNewFiltersFeature(f, eventContexts, filters, eventingv1.TriggerFilter{}, installBroker)

	return f
}
