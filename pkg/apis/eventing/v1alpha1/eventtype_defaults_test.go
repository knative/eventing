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

func TestEventTypeDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  EventType
		expected EventType
	}{
		"nil spec": {
			initial: EventType{},
			expected: EventType{
				Spec: EventTypeSpec{
					Broker: "default",
				},
			},
		},
		"broker empty": {
			initial: EventType{
				Spec: EventTypeSpec{
					Type:   "test-type",
					Source: "test-source",
					Broker: "",
					Schema: "test-schema",
				},
			},
			expected: EventType{
				Spec: EventTypeSpec{
					Type:   "test-type",
					Source: "test-source",
					Broker: "default",
					Schema: "test-schema",
				},
			},
		},
		"broker not set": {
			initial: EventType{
				Spec: EventTypeSpec{
					Type:   "test-type",
					Source: "test-source",
					Schema: "test-schema",
				},
			},
			expected: EventType{
				Spec: EventTypeSpec{
					Type:   "test-type",
					Source: "test-source",
					Broker: "default",
					Schema: "test-schema",
				},
			},
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
