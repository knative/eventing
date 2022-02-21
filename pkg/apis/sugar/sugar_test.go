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

package sugar

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewConfigFromMap(t *testing.T) {
	testCases := []struct {
		name       string
		wantErr    bool
		wantConfig *Config
		data       map[string]string
	}{
		{
			name:    "empty config",
			wantErr: false,
			wantConfig: &Config{
				NamespaceSelector: nil,
				TriggerSelector:   nil,
			},
			data: map[string]string{},
		},
		{
			name:    "enabled namespace",
			wantErr: false,
			wantConfig: &Config{
				NamespaceSelector: &v1.LabelSelector{MatchExpressions: []v1.LabelSelectorRequirement{{
					Key:      "some-key-here",
					Operator: "In",
					Values:   []string{"some-value"},
				}}},
				TriggerSelector: nil,
			},
			data: map[string]string{
				"namespace-selector": `{
					"matchExpressions": [
					  {
						"key": "some-key-here",
						"operator": "In",
						"values": [
						  "some-value"
						]
					  }
					]
				  }`,
			},
		},
		{
			name:    "enabled trigger",
			wantErr: false,
			wantConfig: &Config{
				NamespaceSelector: nil,
				TriggerSelector: &v1.LabelSelector{MatchExpressions: []v1.LabelSelectorRequirement{{
					Key:      "some-key-here",
					Operator: "In",
					Values:   []string{"some-value"},
				}}},
			},
			data: map[string]string{
				"trigger-selector": `{
					"matchExpressions": [
					  {
						"key": "some-key-here",
						"operator": "In",
						"values": [
						  "some-value"
						]
					  }
					]
				  }`,
			},
		},
		{
			name:       "dangling namespace-selector key/val",
			wantErr:    true,
			wantConfig: &Config{},
			data: map[string]string{
				"namespace-selector": `[]`,
			},
		},
		{
			name:       "dangling trigger-selector key/val",
			wantErr:    true,
			wantConfig: &Config{},
			data: map[string]string{
				"trigger-selector": `[]`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualConfig, err := NewConfigFromMap(tc.data)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Test: %q: TestNewConfigFromMap() error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
			if !tc.wantErr {
				// Testing DeepCopy just to increase coverage
				actualConfig = actualConfig.DeepCopy()

				if diff := cmp.Diff(tc.wantConfig, actualConfig); diff != "" {
					t.Error("unexpected value (-want, +got)", diff)
				}
			}
		})
	}
}
