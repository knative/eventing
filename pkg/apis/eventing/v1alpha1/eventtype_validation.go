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
	"context"
	"fmt"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

func (et *EventType) Validate(ctx context.Context) *apis.FieldError {
	return et.Spec.Validate(ctx).ViaField("spec")
}

func (ets *EventTypeSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if ets.Type == "" {
		fe := apis.ErrMissingField("type")
		errs = errs.Also(fe)
	}
	if ets.Importer.Kind == "" {
		fe := apis.ErrMissingField("importer.kind")
		errs = errs.Also(fe)
	}
	if ets.Importer.APIVersion == "" {
		fe := apis.ErrMissingField("importer.apiVersion")
		errs = errs.Also(fe)
	}
	if len(ets.Importer.Parameters) == 0 {
		fe := apis.ErrMissingField("importer.parameters")
		errs = errs.Also(fe)
	} else {
		for i, p := range ets.Importer.Parameters {
			if p.Name == "" {
				fe := apis.ErrMissingField(fmt.Sprintf("importer.parameters[%d].name", i))
				errs = errs.Also(fe)
			}
			if p.Description == "" {
				fe := apis.ErrMissingField(fmt.Sprintf("importer.parameters[%d].description", i))
				errs = errs.Also(fe)
			}
		}
	}
	return errs
}

func (et *EventType) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	if og == nil {
		return nil
	}

	original, ok := og.(*EventType)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not an EventType"}
	}

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
