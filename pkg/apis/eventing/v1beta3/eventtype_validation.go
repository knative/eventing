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
	"strings"

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
	errs = errs.Also(ets.ValidateVariables().ViaField("variables"))
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

func (ets *EventTypeSpec) ValidateVariables() *apis.FieldError {
	var errs *apis.FieldError

	usedVariables := ets.extractAttributeVariables()
	if len(usedVariables) == 0 {
		return nil
	}

	definedVariables := make(map[string]EventVariableDefinition, len(ets.Variables))
	for _, variable := range ets.Variables {
		definedVariables[variable.Name] = variable
	}

	var missingVariables []string
	for _, varName := range usedVariables {
		if _, ok := definedVariables[varName]; !ok {
			// keep track of any used variables that aren't defined
			missingVariables = append(missingVariables, varName)
		}
	}

	if len(missingVariables) > 0 {
		errs = errs.Also(apis.ErrMissingField(missingVariables...))
	}

	return errs
}

// extractEmbeddedAttributeVariables extracts variables embedded within attribute values
// enclosed in curly brackets (e.g. "path.{A}.{B}" -> ["A", "B"]).
func (ets *EventTypeSpec) extractAttributeVariables() []string {
	var variables []string

	for _, attr := range ets.Attributes {
		for idx := 0; idx < len(attr.Value); idx++ {
			if attr.Value[idx] == '\\' {
				idx++ // skip over escaped character
				continue
			}
			if attr.Value[idx] != '{' {
				continue // ignore characters not enclosed in curly brackets
			}

			idx++
			var varName strings.Builder
			for idx < len(attr.Value) && attr.Value[idx] != '}' {
				varName.WriteByte(attr.Value[idx])
				idx++
			}

			if idx < len(attr.Value) && attr.Value[idx] == '}' && varName.Len() > 0 {
				variables = append(variables, varName.String())
			}
		}
	}
	return variables
}
