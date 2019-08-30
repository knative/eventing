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

package metricskey

const (
	// LabelFilterResult is the label for the Trigger filtering result.
	LabelFilterResult = "filter_result"

	// LabelResult is the label for the result of sending an event to a downstream consumer. One of "success", "error".
	LabelResult = "result"

	// AnyValue is the default value if the trigger filter attributes are empty.
	AnyValue = "any"
)
