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

// Parameter represents a named configuration parameter that must be supplied to
// create a particular resource by an Argument. Parameters may optionally have a
// default value that will be used if an Argument is not supplied.
type Parameter struct {
	// Name is the name of the Parameter.
	Name string `json:"name"`

	// Description is the human friendly description of the parameter.
	Description string `json:"description"`

	// Default is the value to use if an Argument for this Parameter is not
	// explicitly set.
	Default *string `json:"default,omitempty"`
}

// Argument represents a value for a named parameter.
type Argument struct {
	// Name is the name of the Parameter this Argument is for.
	Name string `json:"name"`

	// Value is the value for the Parameter.
	Value string `json:"value"`
}
