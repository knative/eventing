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

package testutils

import "knative.dev/eventing/pkg/apis/eventing"

// AnnotationsTransition contains original and current annotations,
// and a flag that is true if the transformation from original
// to current is not valid
type AnnotationsTransition struct {
	Original  map[string]string
	Current   map[string]string
	WantError bool
}

// GetScopeAnnotationsTransitions returns an array of annotations transitions
// useful for testing the validation logic of components that supports
// eventing.knative.dev/scope annotation
func GetScopeAnnotationsTransitions() []AnnotationsTransition {
	return []AnnotationsTransition{
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeCluster},
			Current:   map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeCluster},
			WantError: false,
		},
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeNamespace},
			Current:   map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeCluster},
			WantError: true,
		},
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeNamespace},
			Current:   map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeNamespace},
			WantError: false,
		},
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeCluster},
			Current:   map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeNamespace},
			WantError: true,
		},
		{
			Original:  map[string]string{},
			Current:   map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeNamespace},
			WantError: true,
		},
		{
			Original:  map[string]string{},
			Current:   map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeCluster},
			WantError: false,
		},
		{
			Original:  map[string]string{},
			Current:   map[string]string{},
			WantError: false,
		},
	}
}
