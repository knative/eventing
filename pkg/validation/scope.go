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

	"knative.dev/eventing/pkg/apis/eventing"
)

func Scope(originalAnnotations map[string]string, currentAnnotations map[string]string) error {

	currentScope, currentHasAnnotation := currentAnnotations[eventing.ScopeAnnotationKey]

	originalScope, originalHasAnnotation := originalAnnotations[eventing.ScopeAnnotationKey]

	if !currentHasAnnotation && !originalHasAnnotation {
		return nil
	}

	if !originalHasAnnotation {
		if currentScope != eventing.DefaultScope {
			return fmt.Errorf("'%s' annotation was not present - applied default '%s'", eventing.ScopeAnnotationKey, eventing.DefaultScope)
		}
		return nil
	}

	if originalScope != currentScope {
		return fmt.Errorf("'%s' annotation was '%s' - current is '%s'", eventing.ScopeAnnotationKey, originalScope, currentScope)
	}

	return nil
}