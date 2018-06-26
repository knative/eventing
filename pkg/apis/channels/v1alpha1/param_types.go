/*
 * Copyright 2018 The Knative Authors
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

package v1alpha1

// inspired by the parameter/argument from buildtemplate/build

// Parameter a named parameter that may be defined
type Parameter struct {
	// Name parameter to apply
	Name string `json:"name"`

	// Description human friendly description of the parameter
	Description string `json:"description"`

	// Default (optional) value to use if not explicitly set
	Default *string `json:"default,omitempty"`
}

// Argument a value for a named parameter
type Argument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
