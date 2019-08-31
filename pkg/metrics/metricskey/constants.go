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

	// LabelResponseCode is the label for the HTTP response status code.
	LabelResponseCode = "response_code"

	// LabelResponseCodeClass is the label for the HTTP response status code class. For example, "2xx", "3xx", etc.
	LabelResponseCodeClass = "response_code_class"

	// AnyValue is the default value if the trigger filter attributes are empty.
	AnyValue = "any"
)
