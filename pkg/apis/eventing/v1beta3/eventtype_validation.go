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
	errs = errs.Also(ets.ValidateAttributes())
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
	var errs *apis.FieldError
	if _, ok := ets.Attributes["type"]; !ok {
		errs = errs.Also(&apis.FieldError{
			Message: "type must be set as an attribute",
		})
	}
	if _, ok := ets.Attributes["source"]; !ok {
		errs = errs.Also(&apis.FieldError{
			Message: "source must be set as an attribute",
		})
	}
	if _, ok := ets.Attributes["specversion"]; !ok {
		errs = errs.Also(&apis.FieldError{
			Message: "specversion must be set as an attribute",
		})
	}
	if _, ok := ets.Attributes["id"]; !ok {
		errs = errs.Also(&apis.FieldError{
			Message: "id must be set as an attribute",
		})
	}

	return errs.ViaField("attributes")
}
