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
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	defaultBroker        = "default"
	otherBroker          = "other_broker"
	defaultTriggerFilter = &TriggerFilter{}
	otherTriggerFilter   = &TriggerFilter{
		DeprecatedSourceAndType: &TriggerFilterSourceAndType{
			Type:   "other_type",
			Source: "other_source"},
	}

	defaultTrigger = Trigger{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{brokerLabel: defaultBroker},
		},
		Spec: TriggerSpec{
			Broker: defaultBroker,
			Filter: defaultTriggerFilter,
		},
	}
)

func TestTriggerDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  Trigger
		expected Trigger
	}{
		"nil broker": {
			initial: Trigger{Spec: TriggerSpec{Filter: otherTriggerFilter}},
			expected: Trigger{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{brokerLabel: defaultBroker},
				},
				Spec: TriggerSpec{Broker: defaultBroker, Filter: otherTriggerFilter}},
		},
		"nil filter": {
			initial: Trigger{Spec: TriggerSpec{Broker: otherBroker}},
			expected: Trigger{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{brokerLabel: otherBroker},
				},
				Spec: TriggerSpec{Broker: otherBroker, Filter: defaultTriggerFilter}},
		},
		"nil broker and nil filter": {
			initial:  Trigger{},
			expected: defaultTrigger,
		},
		"with broker and label": {
			initial: Trigger{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{"otherLabel": "my-other-label"},
				},
				Spec: TriggerSpec{Broker: defaultBroker}},
			expected: Trigger{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"otherLabel": "my-other-label",
						brokerLabel:  defaultBroker},
				},
				Spec: TriggerSpec{Broker: defaultBroker, Filter: defaultTriggerFilter}},
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
