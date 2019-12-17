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
	"regexp"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

var (
	// Only allow lowercase alphanumeric, starting with letters.
	validSelectorName = regexp.MustCompile(`^[a-z][a-z0-9./-]*$`)
)

// Validate the ConfigMapPropagation.
func (cmp *ConfigMapPropagation) Validate(ctx context.Context) *apis.FieldError {
	return cmp.Spec.Validate(ctx).ViaField("spec")
}

// Validate the ConfigMapPropagationSpec.
func (cmps *ConfigMapPropagationSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if cmps.OriginalNamespace == "" {
		fe := apis.ErrMissingField("originalNamespace")
		errs = errs.Also(fe)
	}
	if cmps.Selector == nil {
		fe := apis.ErrMissingField("selector")
		errs = errs.Also(fe)
	}

	if cmps.Selector != nil {
		if len(cmps.Selector) == 0 {
			fe := &apis.FieldError{
				Message: "At least one selector must be specified",
				Paths:   []string{"selector"},
			}
			errs = errs.Also(fe)
		} else {
			for s := range cmps.Selector {
				if !validSelectorName.MatchString(s) {
					fe := &apis.FieldError{
						Message: fmt.Sprintf("Invalid selector name: %q", s),
						Paths:   []string{"selector"},
					}
					errs = errs.Also(fe)
				}
			}
		}
	}
	return errs
}

// CheckImmutableFields checks that any immutable fields were not changed.
func (cmp *ConfigMapPropagation) CheckImmutableFields(ctx context.Context, original *ConfigMapPropagation) *apis.FieldError {
	if original == nil {
		return nil
	}

	if diff, err := kmp.ShortDiff(original.Spec, cmp.Spec); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff ConfigMapPropagation",
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
