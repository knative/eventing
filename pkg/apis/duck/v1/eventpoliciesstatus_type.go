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

// AppliedEventPoliciesStatus contains the list of policies which apply to a resource.
// This type is intended to be embedded into a status struct.
type AppliedEventPoliciesStatus struct {
	// Policies holds the list of applied EventPolicies
	// +optional
	Policies []AppliedEventPolicyRef `json:"policies,omitempty"`
}

// AppliedEventPolicyRef is the reference to an EventPolicy
type AppliedEventPolicyRef struct {
	// APIVersion of the applied EventPolicy.
	// This indicates, which version of EventPolicy is supported by the resource.
	APIVersion string `json:"apiVersion"`

	// Name of the applied EventPolicy
	Name string `json:"name"`
}
