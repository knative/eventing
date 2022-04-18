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
	"knative.dev/reconciler-test/pkg/eventshub"
	. "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"

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
			trigger.WithSubscriber(svc.AsKReference(subscriber), ""),
			WithNewFilters(fs.filters),
		}

		f.Setup("Install trigger", trigger.Install(triggerName, brokerName, cfg...))
		f.Setup("Wait for trigger to become ready", trigger.IsReady(triggerName))

		f.Requirement("Install matched event sender", func(ctx context.Context, t feature.T) {
			u, err := broker.Address(ctx, brokerName)
			if err != nil || u == nil {
				t.Error("failed to get the address of the broker", brokerName, err)
			}
			eventshub.Install(matchedSender, eventshub.StartSenderURL(u.String()), eventshub.InputEvent(matchedEvent))(ctx, t)
		})

		f.Requirement("Install unmatched event sender", func(ctx context.Context, t feature.T) {
			u, err := broker.Address(ctx, brokerName)
			if err != nil || u == nil {
				t.Error("failed to get the address of the broker", brokerName, err)
			}
			eventshub.Install(unmatchedSender, eventshub.StartSenderURL(u.String()), eventshub.InputEvent(unmatchedEvent))(ctx, t)
		})

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
    - %s:
        source: %%s`, key, key)
}

// WithNewFilters adds a filter config to a Trigger spec using the new filters API.
func WithNewFilters(filters string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["filters"] = filters
	}
}
