/*
Copyright 2019 The Knative Authors

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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	defaultBroker = "default"
	otherBroker   = "other_broker"

	otherTriggerFilter = &TriggerFilter{
		SourceAndType: &TriggerFilterSourceAndType{
			Type:   "other_type",
			Source: "other_source"},
	}
	defaultTrigger = Trigger{
		Spec: TriggerSpec{
			Broker: defaultBroker,
			Filter: defaultTriggerFilter(),
		},
	}
)

func TestTriggerDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  Trigger
		expected Trigger
	}{
		"nil broker": {
			initial:  Trigger{Spec: TriggerSpec{Filter: otherTriggerFilter}},
			expected: Trigger{Spec: TriggerSpec{Broker: defaultBroker, Filter: otherTriggerFilter}},
		},
		"nil filter": {
			initial:  Trigger{Spec: TriggerSpec{Broker: otherBroker}},
			expected: Trigger{Spec: TriggerSpec{Broker: otherBroker, Filter: defaultTriggerFilter()}},
		},
		"nil broker and nil filter": {
			initial:  Trigger{},
			expected: defaultTrigger,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.TODO())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}

func defaultTriggerFilter() *TriggerFilter {
	// Can't just be a package level var because it gets mutated.
	return &TriggerFilter{
		SourceAndType: &TriggerFilterSourceAndType{
			Type:   TriggerAnyFilter,
			Source: TriggerAnyFilter},
	}
}
