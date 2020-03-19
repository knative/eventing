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

import (
	"context"
	"fmt"
	"testing"

	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/eventing"
)

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
func GetScopeAnnotationsTransitions(defaultScope, otherScope string) []AnnotationsTransition {
	return []AnnotationsTransition{
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: defaultScope},
			Current:   map[string]string{eventing.ScopeAnnotationKey: defaultScope},
			WantError: false,
		},
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: otherScope},
			Current:   map[string]string{eventing.ScopeAnnotationKey: defaultScope},
			WantError: true,
		},
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: otherScope},
			Current:   map[string]string{eventing.ScopeAnnotationKey: otherScope},
			WantError: false,
		},
		{
			Original:  map[string]string{eventing.ScopeAnnotationKey: defaultScope},
			Current:   map[string]string{eventing.ScopeAnnotationKey: otherScope},
			WantError: true,
		},
		{
			Original:  map[string]string{},
			Current:   map[string]string{eventing.ScopeAnnotationKey: otherScope},
			WantError: true,
		},
		{
			Original:  map[string]string{},
			Current:   map[string]string{eventing.ScopeAnnotationKey: defaultScope},
			WantError: false,
		},
		{
			Original:  map[string]string{},
			Current:   map[string]string{},
			WantError: false,
		},
	}
}

type Resource interface {
	GetAnnotations() map[string]string
	apis.Validatable
}

type ResourceCreator func(map[string]string) Resource

// CheckScopeAnnotationWithTransitions check if the given transitions are valid for resources created by creator
func CheckScopeAnnotationWithTransitions(t *testing.T, creator ResourceCreator, transitions []AnnotationsTransition) {

	type tc struct {
		name      string
		ctx       context.Context
		r         Resource
		wantError bool
	}

	tt := make([]tc, len(transitions))

	for i, t := range transitions {
		tt[i] = tc{
			name:      fmt.Sprintf("original %s current %s", t.Original[eventing.ScopeAnnotationKey], t.Current[eventing.ScopeAnnotationKey]),
			ctx:       apis.WithinUpdate(context.TODO(), creator(t.Original)),
			r:         creator(t.Current),
			wantError: t.WantError,
		}
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.r.Validate(tc.ctx)
			if tc.wantError != (errs != nil) {
				t.Fatalf("want error %v got %+v", tc.wantError, errs)
			}
		})
	}
}
