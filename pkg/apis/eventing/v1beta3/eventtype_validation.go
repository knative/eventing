/*
Copyright 2023 The Knative Authors

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

package v1beta3

import (
	"context"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

func (et *EventType) Validate(ctx context.Context) *apis.FieldError {
	return et.Spec.Validate(ctx).ViaField("spec")
}

func (ets *EventTypeSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	// TODO: validate attribute with name=source is a valid URI
	// TODO: validate attribute with name=schema is a valid URI
	errs = errs.Also(ets.ValidateAttributes().ViaField("attributes"))
	return errs
}

func (et *EventType) CheckImmutableFields(ctx context.Context, original *EventType) *apis.FieldError {
	if original == nil {
		return nil
	}

	// All fields are immutable.
	if diff, err := kmp.ShortDiff(original.Spec, et.Spec); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff EventType",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}

func (ets *EventTypeSpec) ValidateAttributes() *apis.FieldError {
	attributes := make(map[string]EventAttributeDefinition, len(ets.Attributes))
	for _, attr := range ets.Attributes {
		attributes[attr.Name] = attr
	}

	missingFields := []string{}
	if _, ok := attributes["type"]; !ok {
		missingFields = append(missingFields, "type")
	}
	if _, ok := attributes["source"]; !ok {
		missingFields = append(missingFields, "source")
	}
	if _, ok := attributes["specversion"]; !ok {
		missingFields = append(missingFields, "specversion")
	}
	if _, ok := attributes["id"]; !ok {
		missingFields = append(missingFields, "id")
	}

	if len(missingFields) > 0 {
		return apis.ErrMissingField(missingFields...)
	}

	return nil
}
