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
	"k8s.io/apimachinery/pkg/util/validation"
)

func (et *EventType) Validate() *apis.FieldError {
	return et.Spec.Validate().ViaField("spec")
}

func (ets *EventTypeSpec) Validate() *apis.FieldError {
	if ets.EventSource == "" {
		return apis.ErrMissingField("eventSource")
	}
	if errs := validation.IsQualifiedName(ets.EventSource); len(errs) > 0 {
		return apis.ErrInvalidValue(ets.EventSource, "eventSource")
	}
	return ets.CommonEventTypeSpec.Validate()
}

func (current *EventType) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	original, ok := og.(*EventType)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a EventType"}
	}
	if original == nil {
		return nil
	}

	// EventSource for an EventType should not change.
	if original.Spec.EventSource != current.Spec.EventSource {
		return &apis.FieldError{
			Message: "Immutable fields changed",
			Paths:   []string{"spec.eventSource"},
		}
	}

	return nil
}
