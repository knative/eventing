/*
Copyright 2020 The Knative Authors

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

package resources

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestOriginalLabels(t *testing.T) {
	testCases := []struct {
		Name string
		F    func() labels.Selector
		Want labels.Selector
	}{{
		Name: "empty map",
		F: func() labels.Selector {
			return ExpectedOriginalSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{},
			})
		},
		Want: labels.SelectorFromSet(map[string]string{
			PropagationLabelKey: PropagationLabelValueOriginal,
		}),
	}, {
		Name: "map with existing keys",
		F: func() labels.Selector {
			return ExpectedOriginalSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"testing": "testing",
				},
			})
		},
		Want: labels.SelectorFromSet(map[string]string{
			"testing":           "testing",
			PropagationLabelKey: PropagationLabelValueOriginal,
		}),
	}, {
		Name: "map with original key",
		F: func() labels.Selector {
			return ExpectedOriginalSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"testing":           "testing",
					PropagationLabelKey: PropagationLabelValueOriginal,
				},
			})
		},
		Want: labels.SelectorFromSet(map[string]string{
			"testing":           "testing",
			PropagationLabelKey: PropagationLabelValueOriginal,
		}),
	}}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := tc.F(); !reflect.DeepEqual(got, tc.Want) {
				t.Errorf("want %v, got %v", tc.Want, got)
			}
		})
	}
}
