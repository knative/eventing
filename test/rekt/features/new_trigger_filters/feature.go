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
	"knative.dev/reconciler-test/pkg/feature"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
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
		f = newEventFilterFeature(eventContexts, fs.filters, f, brokerName)
		features = append(features, f)
	}

	return &feature.FeatureSet{
		Name:     "New trigger filters",
		Features: features,
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
						CESQL: "type LIKE '%event.type%'",
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

func AllFilterFeature(brokerName string) *feature.Feature {
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

	f = newEventFilterFeature(eventContexts, filters, f, brokerName)

	return f
}

func MultipleTriggersAndSinksFeature(brokerName string) *feature.Feature {
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

	f = newEventFilterFeature(eventContextsFirstSink, filtersFirstTrigger, f, brokerName)
	f = newEventFilterFeature(eventContextsSecondSink, filtersSecondTrigger, f, brokerName)

	return f
}
