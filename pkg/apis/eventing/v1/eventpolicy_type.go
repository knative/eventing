/*
Copyright 2024 The Knative Authors

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

package v1

// AppliedEventPoliciesStatus contains the list of policies which apply to a resource
type AppliedEventPoliciesStatus struct {
	// Policies holds the list of applied EventPolicies
	// +optional
	Policies []AppliedEventPoliciesStatusPolicy `json:"policies,omitempty"`
}

// AppliedEventPoliciesStatusPolicy is the reference to a EventPolicy
type AppliedEventPoliciesStatusPolicy struct {
	// APIVersion of the applied EventPolicy.
	// This indicates, which version of EventPolicy is supported by the resource.
	APIVersion string `json:"apiVersion"`

	// Name of the applied EventPolicy
	Name string `json:"name"`
}
