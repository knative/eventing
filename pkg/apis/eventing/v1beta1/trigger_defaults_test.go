/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1beta1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
)

var (
	defaultBroker      = "default"
	otherBroker        = "other_broker"
	namespace          = "testnamespace"
	emptyTriggerFilter = &TriggerFilter{}
	defaultTrigger     = Trigger{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{brokerLabel: defaultBroker},
		},
		Spec: TriggerSpec{
			Broker: defaultBroker,
			Filter: emptyTriggerFilter,
		},
	}
)

func TestTriggerDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  Trigger
		expected Trigger
	}{
		"nil broker": {
			initial: Trigger{Spec: TriggerSpec{Filter: emptyTriggerFilter}},
			expected: Trigger{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{brokerLabel: defaultBroker},
				},
				Spec: TriggerSpec{Broker: defaultBroker, Filter: emptyTriggerFilter}},
		},
		"nil filter": {
			initial: Trigger{Spec: TriggerSpec{Broker: otherBroker}},
			expected: Trigger{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{brokerLabel: otherBroker},
				},
				Spec: TriggerSpec{Broker: otherBroker, Filter: emptyTriggerFilter}},
		},
		"subscriber, ns defaulted": {
			initial: Trigger{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
				},
				Spec: TriggerSpec{
					Broker: otherBroker,
					Subscriber: duckv1.Destination{
						Ref: &duckv1.KReference{
							Name: "foo",
						},
					}}},
			expected: Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: namespace,
					Labels:    map[string]string{brokerLabel: otherBroker},
				},
				Spec: TriggerSpec{
					Broker: otherBroker,
					Filter: emptyTriggerFilter,
					Subscriber: duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:      "foo",
							Namespace: namespace,
						},
					},
				}},
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
				Spec: TriggerSpec{Broker: defaultBroker, Filter: emptyTriggerFilter}},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.TODO())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatal("Unexpected defaults (-want, +got):", diff)
			}
		})
	}
}
