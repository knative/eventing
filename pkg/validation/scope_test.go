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
	"knative.dev/eventing/pkg/testutils"
	"testing"
)

type r struct {
	annotations map[string]string
}

func (m r) GetAnnotations() map[string]string {
	return m.annotations
}

func TestScope(t *testing.T) {

	tt := testutils.GetScopeAnnotationsTransitions(eventing.ScopeCluster, eventing.ScopeNamespace)

	for _, tc := range tt {
		name := fmt.Sprintf("original %s current %s", tc.Original[eventing.ScopeAnnotationKey], tc.Current[eventing.ScopeAnnotationKey])
		t.Run(name, func(t *testing.T) {
			err := Scope(nil, r{tc.Original}, r{tc.Current}, eventing.ScopeCluster)
			if tc.WantError != (err != nil) {
				t.Fatalf("want error %v got error %+v", tc.WantError, err)
			}
		})
	}
}
