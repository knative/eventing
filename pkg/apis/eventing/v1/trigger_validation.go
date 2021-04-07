/*
Copyright 2020 The Knative Authors

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

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"

	corev1 "k8s.io/api/core/v1"
)

var (
	// Only allow lowercase alphanumeric, starting with letters.
	validAttributeName = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
)

// Validate the Trigger.
func (t *Trigger) Validate(ctx context.Context) *apis.FieldError {
	errs := t.Spec.Validate(ctx).ViaField("spec")
	errs = t.validateAnnotation(errs, DependencyAnnotation, t.validateDependencyAnnotation)
	errs = t.validateAnnotation(errs, InjectionAnnotation, t.validateInjectionAnnotation)
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Trigger)
		errs = errs.Also(t.CheckImmutableFields(ctx, original))
	}
	return errs
}

// Validate the TriggerSpec.
func (ts *TriggerSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if ts.Broker == "" {
		fe := apis.ErrMissingField("broker")
		errs = errs.Also(fe)
	}

	for _, err := range validateFilterSpec(ts.Filter, []string{"filter"}) {
		errs = errs.Also(err)
	}

	if fe := ts.Subscriber.Validate(ctx); fe != nil {
		errs = errs.Also(fe.ViaField("subscriber"))
	}

	if ts.Delivery != nil {
		if de := ts.Delivery.Validate(ctx); de != nil {
			errs = errs.Also(de.ViaField("delivery"))
		}
	}

	return errs
}

func validateFilterSpec(filter *FilterSpec, path []string) (errs []*apis.FieldError) {
	if filter == nil {
		return nil
	}

	// Validate Attributes
	for attr := range map[string]string(filter.Attributes) {
		if !validAttributeName.MatchString(attr) {
			errs = append(errs, &apis.FieldError{
				Message: fmt.Sprintf("Invalid attribute name: %q", attr),
				Paths:   []string{strings.Join(append(path, "attributes"), ".")},
			})
		}
	}

	// Validate Exact
	if filter.Exact != nil {
		if len(filter.Exact) != 1 {
			errs = append(errs, &apis.FieldError{
				Message: fmt.Sprintf("Exact can have only one key-value"),
				Paths:   []string{strings.Join(append(path, "exact"), ".")},
			})
		}
		for attr := range filter.Exact {
			if !validAttributeName.MatchString(attr) {
				errs = append(errs, &apis.FieldError{
					Message: fmt.Sprintf("Invalid attribute name: %q", attr),
					Paths:   []string{strings.Join(append(path, "exact"), ".")},
				})
			}
		}
	}

	// Validate Prefix
	if filter.Prefix != nil {
		if len(filter.Prefix) != 1 {
			errs = append(errs, &apis.FieldError{
				Message: "Prefix can have only one key-value",
				Paths:   []string{strings.Join(append(path, "prefix"), ".")},
			})
		}
		for attr := range filter.Prefix {
			if !validAttributeName.MatchString(attr) {
				errs = append(errs, &apis.FieldError{
					Message: fmt.Sprintf("Invalid attribute name: %q", attr),
					Paths:   []string{strings.Join(append(path, "prefix"), ".")},
				})
			}
		}
	}

	// Validate Suffix
	if filter.Suffix != nil {
		if len(filter.Suffix) != 1 {
			errs = append(errs, &apis.FieldError{
				Message: "Suffix can have only one key-value",
				Paths:   []string{strings.Join(append(path, "suffix"), ".")},
			})
		}
		for attr := range filter.Suffix {
			if !validAttributeName.MatchString(attr) {
				errs = append(errs, &apis.FieldError{
					Message: fmt.Sprintf("Invalid attribute name: %q", attr),
					Paths:   []string{strings.Join(append(path, "suffix"), ".")},
				})
			}
		}
	}

	// Validate All
	if filter.All != nil {
		if len(filter.All) < 1 {
			errs = append(errs, &apis.FieldError{
				Message: "All must contain at least one nested filter",
				Paths:   []string{strings.Join(append(path, "all"), ".")},
			})
		}

		for i, f := range filter.All {
			f := f
			errs = append(errs, validateFilterSpec(&f, append(path, "all", fmt.Sprintf("[%d]", i)))...)
		}
	}

	// Validate Any
	if filter.Any != nil {
		if len(filter.Any) < 1 {
			errs = append(errs, &apis.FieldError{
				Message: "Any must contain at least one nested filter",
				Paths:   []string{strings.Join(append(path, "any"), ".")},
			})
		}

		for i, f := range filter.Any {
			f := f
			errs = append(errs, validateFilterSpec(&f, append(path, "any", fmt.Sprintf("[%d]", i)))...)
		}
	}

	// Validate Not
	if filter.Not != nil {
		errs = append(errs, validateFilterSpec(filter.Not, append(path, "not"))...)
	}

	return
}

// CheckImmutableFields checks that any immutable fields were not changed.
func (t *Trigger) CheckImmutableFields(ctx context.Context, original *Trigger) *apis.FieldError {
	if original == nil {
		return nil
	}

	if diff, err := kmp.ShortDiff(original.Spec.Broker, t.Spec.Broker); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Trigger",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec", "broker"},
			Details: diff,
		}
	}
	return nil
}

func GetObjRefFromDependencyAnnotation(dependencyAnnotation string) (corev1.ObjectReference, error) {
	var objectRef corev1.ObjectReference
	if err := json.Unmarshal([]byte(dependencyAnnotation), &objectRef); err != nil {
		return objectRef, err
	}
	return objectRef, nil
}

func (t *Trigger) validateAnnotation(errs *apis.FieldError, annotation string, function func(string) *apis.FieldError) *apis.FieldError {
	if annotationValue, ok := t.GetAnnotations()[annotation]; ok {
		annotationPrefix := fmt.Sprintf("metadata.annotations[%s]", annotation)
		errs = errs.Also(function(annotationValue).ViaField(annotationPrefix))
	}
	return errs
}

func (t *Trigger) validateDependencyAnnotation(dependencyAnnotation string) *apis.FieldError {
	depObjRef, err := GetObjRefFromDependencyAnnotation(dependencyAnnotation)
	if err != nil {
		return &apis.FieldError{
			Message: fmt.Sprintf("The provided annotation was not a corev1.ObjectReference: %q", dependencyAnnotation),
			Details: err.Error(),
			Paths:   []string{""},
		}
	}
	var errs *apis.FieldError
	if depObjRef.Namespace != "" && depObjRef.Namespace != t.GetNamespace() {
		fe := &apis.FieldError{
			Message: fmt.Sprintf("Namespace must be empty or equal to the trigger namespace %q", t.GetNamespace()),
			Paths:   []string{"namespace"},
		}
		errs = errs.Also(fe)
	}
	if depObjRef.Kind == "" {
		fe := apis.ErrMissingField("kind")
		errs = errs.Also(fe)
	}
	if depObjRef.Name == "" {
		fe := apis.ErrMissingField("name")
		errs = errs.Also(fe)
	}
	if depObjRef.APIVersion == "" {
		fe := apis.ErrMissingField("apiVersion")
		errs = errs.Also(fe)
	}
	return errs
}

func (t *Trigger) validateInjectionAnnotation(injectionAnnotation string) *apis.FieldError {
	if injectionAnnotation != "enabled" && injectionAnnotation != "disabled" {
		return &apis.FieldError{
			Message: fmt.Sprintf(`The provided injection annotation value can only be "enabled" or "disabled", not %q`, injectionAnnotation),
			Paths:   []string{""},
		}
	}
	if t.Spec.Broker != "default" {
		return &apis.FieldError{
			Message: fmt.Sprintf("The provided injection annotation is only used for default broker, but non-default broker specified here: %q", t.Spec.Broker),
			Paths:   []string{""},
		}
	}
	return nil
}
