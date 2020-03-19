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

package validation

import (
	"fmt"

	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/eventing"
)

type AnnotationProvider interface {
	GetAnnotations() map[string]string
}

func Scope(errs *apis.FieldError, original AnnotationProvider, current AnnotationProvider, defaultScope string) *apis.FieldError {

	currentScope, currentHasAnnotation := current.GetAnnotations()[eventing.ScopeAnnotationKey]

	originalScope, originalHasAnnotation := original.GetAnnotations()[eventing.ScopeAnnotationKey]

	if !currentHasAnnotation && !originalHasAnnotation {
		return nil
	}

	if !originalHasAnnotation {
		if currentScope != defaultScope {
			return addError(errs, &apis.FieldError{
				Message: fmt.Sprintf("invalid %s annotation value", eventing.ScopeAnnotationKey),
				Paths:   []string{"metadata.annotations"},
				Details: fmt.Sprintf("'%s' annotation was not present - applied default '%s'", eventing.ScopeAnnotationKey, defaultScope),
			})
		}
		return nil
	}

	if originalScope != currentScope {
		return addError(errs, &apis.FieldError{
			Message: fmt.Sprintf("invalid %s annotation value", eventing.ScopeAnnotationKey),
			Paths:   []string{"metadata.annotations"},
			Details: fmt.Sprintf("'%s' annotation was '%s' - current is '%s'", eventing.ScopeAnnotationKey, originalScope, currentScope),
		})
	}

	return errs
}

func addError(errs *apis.FieldError, err *apis.FieldError) *apis.FieldError {
	if errs == nil {
		errs = err
		return errs
	}
	return errs.Also(err)
}
