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

	cesqlparser "github.com/cloudevents/sdk-go/sql/v2/parser"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"

	"knative.dev/eventing/pkg/apis/feature"
)

var (
	// Only allow lowercase alphanumeric, starting with letters.
	validAttributeName = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
)

// Validate the Trigger.
func (t *Trigger) Validate(ctx context.Context) *apis.FieldError {
	errs := t.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec")
	errs = t.validateAnnotation(errs, DependencyAnnotation, t.validateDependencyAnnotation)
	errs = t.validateAnnotation(errs, InjectionAnnotation, t.validateInjectionAnnotation)
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Trigger)
		errs = errs.Also(t.CheckImmutableFields(ctx, original))
	}
	return errs
}

// Validate the TriggerSpec.
func (ts *TriggerSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ts.Broker == "" {
		errs = errs.Also(apis.ErrMissingField("broker"))
	}

	return errs.Also(
		ValidateAttributeFilters(ts.Filter).ViaField("filter"),
	).Also(
		ValidateSubscriptionAPIFiltersList(ctx, ts.Filters).ViaField("filters"),
	).Also(
		ts.Subscriber.Validate(ctx).ViaField("subscriber"),
	).Also(
		ts.Delivery.Validate(ctx).ViaField("delivery"),
	)
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

func ValidateAttributeFilters(filter *TriggerFilter) (errs *apis.FieldError) {
	if filter == nil {
		return nil
	}
	return errs.Also(ValidateAttributesNames(filter.Attributes).ViaField("attributes"))
}

func ValidateAttributesNames(attrs map[string]string) (errs *apis.FieldError) {
	for attr := range attrs {
		if !validAttributeName.MatchString(attr) {
			errs = errs.Also(apis.ErrInvalidKeyName(attr, apis.CurrentField, "Attribute name must start with a letter and can only contain lowercase alphanumeric").ViaKey(attr))
		}
	}
	return errs
}

func ValidateSingleAttributeMap(expr map[string]string) (errs *apis.FieldError) {
	if len(expr) == 0 {
		return nil
	}

	if len(expr) != 1 {
		return apis.ErrGeneric("Multiple items found, can have only one key-value", apis.CurrentField)
	}
	for attr := range expr {
		if !validAttributeName.MatchString(attr) {
			errs = errs.Also(apis.ErrInvalidKeyName(attr, apis.CurrentField, "Attribute name must start with a letter and can only contain lowercase alphanumeric").ViaKey(attr))
		}
	}
	return errs
}

func ValidateSubscriptionAPIFiltersList(ctx context.Context, filters []SubscriptionsAPIFilter) (errs *apis.FieldError) {
	if filters == nil || !feature.FromContext(ctx).IsEnabled(feature.NewTriggerFilters) {
		return nil
	}

	for i, f := range filters {
		f := f
		errs = errs.Also(ValidateSubscriptionAPIFilter(ctx, &f)).ViaIndex(i)
	}
	return errs
}

func ValidateCESQLExpression(ctx context.Context, expression string) (errs *apis.FieldError) {
	if expression == "" {
		return nil
	}
	if _, err := cesqlparser.Parse(expression); err != nil {
		return apis.ErrInvalidValue(expression, apis.CurrentField, err.Error())
	}
	return nil
}

func ValidateSubscriptionAPIFilter(ctx context.Context, filter *SubscriptionsAPIFilter) (errs *apis.FieldError) {
	if filter == nil {
		return nil
	}
	errs = errs.Also(
		ValidateOneOf(filter),
	).Also(
		ValidateSingleAttributeMap(filter.Exact).ViaField("exact"),
	).Also(
		ValidateSingleAttributeMap(filter.Prefix).ViaField("prefix"),
	).Also(
		ValidateSingleAttributeMap(filter.Suffix).ViaField("suffix"),
	).Also(
		ValidateSubscriptionAPIFiltersList(ctx, filter.All).ViaField("all"),
	).Also(
		ValidateSubscriptionAPIFiltersList(ctx, filter.Any).ViaField("any"),
	).Also(
		ValidateSubscriptionAPIFilter(ctx, filter.Not).ViaField("not"),
	).Also(
		ValidateCESQLExpression(ctx, filter.SQL).ViaField("sql"),
	)
	return errs
}

func ValidateOneOf(filter *SubscriptionsAPIFilter) (err *apis.FieldError) {
	if filter != nil && hasMultipleDialects(filter) {
		return apis.ErrGeneric("multiple dialects found, filters can have only one dialect set")
	}
	return nil
}

func hasMultipleDialects(filter *SubscriptionsAPIFilter) bool {
	dialectFound := false
	if len(filter.Exact) > 0 {
		dialectFound = true
	}
	if len(filter.Prefix) > 0 {
		if dialectFound {
			return true
		} else {
			dialectFound = true
		}
	}
	if len(filter.Suffix) > 0 {
		if dialectFound {
			return true
		} else {
			dialectFound = true
		}
	}
	if len(filter.All) > 0 {
		if dialectFound {
			return true
		} else {
			dialectFound = true
		}
	}
	if len(filter.Any) > 0 {
		if dialectFound {
			return true
		} else {
			dialectFound = true
		}
	}
	if filter.Not != nil {
		if dialectFound {
			return true
		} else {
			dialectFound = true
		}
	}
	if filter.SQL != "" && dialectFound {
		return true
	}
	return false
}
