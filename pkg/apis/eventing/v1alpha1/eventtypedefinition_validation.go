package v1alpha1

import (
	"context"
	"knative.dev/pkg/apis"
)

func (et *EventTypeDefinition) Validate(ctx context.Context) *apis.FieldError {
	return et.Spec.Validate(ctx).ViaField("spec")
}

func (ets *EventTypeDefinitionSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	// if ets.Type == "" {
	// 	fe := apis.ErrMissingField("type")
	// 	errs = errs.Also(fe)
	// }
	// TODO validate Source is a valid URI.
	// TODO validate Schema is a valid URI.
	// There is no validation of the SchemaData, it is application specific data.
	return errs
}

func (et *EventTypeDefinition) CheckImmutableFields(ctx context.Context, original *EventTypeDefinition) *apis.FieldError {
	// if original == nil {
	// 	return nil
	// }

	// // All fields are immutable.
	// if diff, err := kmp.ShortDiff(original.Spec, et.Spec); err != nil {
	// 	return &apis.FieldError{
	// 		Message: "Failed to diff EventType",
	// 		Paths:   []string{"spec"},
	// 		Details: err.Error(),
	// 	}
	// } else if diff != "" {
	// 	return &apis.FieldError{
	// 		Message: "Immutable fields changed (-old +new)",
	// 		Paths:   []string{"spec"},
	// 		Details: diff,
	// 	}
	// }
	return nil
}
