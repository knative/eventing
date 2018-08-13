/*
Copyright 2018 The Knative Authors

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
	"github.com/knative/pkg/apis"
)

func (es *EventSource) Validate() *apis.FieldError {
	return es.Spec.Validate().ViaField("spec")
}

func (ess *EventSourceSpec) Validate() *apis.FieldError {
	return ess.CommonEventSourceSpec.Validate()
}

func (current *EventSource) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	original, ok := og.(*EventSource)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a EventSource"}
	}
	if original == nil {
		return nil
	}

	// TODO

	return nil
}
