package experimental

import (
	"context"
	"fmt"
	"reflect"

	"knative.dev/pkg/apis"
)

// ValidateAPIFields checks that the experimental features fields are disabled if the experimental flag is disabled
func ValidateAPIFields(ctx context.Context, featureName string, object interface{}, experimentalFields ...string) (errs *apis.FieldError) {
	obj := reflect.ValueOf(object)
	obj = reflect.Indirect(obj)
	if obj.Kind() != reflect.Struct {
		return nil
	}

	// If feature not enabled, let's check the field is not used
	if !FromContext(ctx).IsEnabled(featureName) {
		for _, fieldName := range experimentalFields {
			fieldVal := obj.FieldByName(fieldName)

			if (fieldVal.Kind() == reflect.Ptr || fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Map) && !fieldVal.IsNil() {
				errs = errs.Also(&apis.FieldError{
					Message: fmt.Sprintf("Disallowed field because the experimental feature '%s' is disabled", featureName),
					Paths:   []string{fmt.Sprintf("%s.%s", obj.Type().Name(), fieldName)},
				})
			}
		}
	}

	return errs
}
