/*
Copyright 2021 The Knative Authors

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

package feature

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// ValidateAPIFields checks that the experimental features fields are disabled if the experimental flag is disabled.
// experimentalFields can contain a string with dots, to identify sub-structs, like "Destination.Ref.APIVersion"
func ValidateAPIFields(ctx context.Context, featureName string, object interface{}, experimentalFields ...string) (errs *apis.FieldError) {
	obj := reflect.ValueOf(object)
	obj = reflect.Indirect(obj)
	if obj.Kind() != reflect.Struct {
		return nil
	}

	// If feature not enabled, let's check the field is not used
	if !FromContext(ctx).IsEnabled(featureName) {
		for _, fieldName := range experimentalFields {
			fieldVal := walk(obj, strings.Split(fieldName, ".")...)

			if !fieldVal.IsZero() {
				errs = errs.Also(&apis.FieldError{
					Message: fmt.Sprintf("Disallowed field because the experimental feature '%s' is disabled", featureName),
					Paths:   []string{fmt.Sprintf("%s.%s", obj.Type().Name(), fieldName)},
				})
			}
		}
	}

	return errs
}

// ValidateAnnotations checks that the experimental features annotations are disabled if the experimental flag is disabled
func ValidateAnnotations(ctx context.Context, featureName string, object metav1.Object, experimentalAnnotations ...string) (errs *apis.FieldError) {
	// If feature not enabled, let's check the annotation is not used
	if !FromContext(ctx).IsEnabled(featureName) {
		for _, annotation := range experimentalAnnotations {
			if _, ok := object.GetAnnotations()[annotation]; ok {
				errs = errs.Also(&apis.FieldError{
					Message: fmt.Sprintf("Disallowed annotation because the experimental feature '%s' is disabled", featureName),
					Paths:   []string{annotation},
				})
			}
		}
	}

	return errs
}

func walk(value reflect.Value, paths ...string) reflect.Value {
	switch value.Kind() {
	case reflect.Struct:
		newVal := value.FieldByName(paths[0])
		if len(paths) == 1 {
			return newVal
		}
		return walk(value.FieldByName(paths[0]), paths[1:]...)
	case reflect.Ptr:
		return walk(reflect.Indirect(value), paths...)
	default:
		return reflect.Zero(value.Type())
	}
}
