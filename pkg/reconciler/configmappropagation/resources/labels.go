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

import "k8s.io/apimachinery/pkg/labels"

const (
	PropagationLabelKey           = "knative.dev/config-propagation"
	PropagationLabelValueOriginal = "original"
	PropagationLabelValueCopy     = "copy"
	CopyLabelKey                  = "knative.dev/config-original"
)

// ExpectedOriginalSelector with return a selector which matches the input set with an additional label "knative.dev/config-propagation: original"
// This is the expected selector to select original configmaps
func ExpectedOriginalSelector(selector map[string]string) labels.Selector {
	expectedOriginalSelector := map[string]string{}
	for index, element := range selector {
		expectedOriginalSelector[index] = element
	}
	// Add original label if it doesn't exist
	if expectedOriginalSelector[PropagationLabelKey] == "" {
		expectedOriginalSelector[PropagationLabelKey] = PropagationLabelValueOriginal
	}
	return labels.SelectorFromSet(expectedOriginalSelector)
}
