/*
Copyright 2019 The Knative Authors

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

package v1alpha1

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
	"testing"
)

var (
	originalNamespace = "original"
	currentNamespace  = "current"
	validSelector     = map[string]string{
		"testing": "testing",
	}
)

func TestConfigMapPropagationValidation(t *testing.T) {
	tests := []struct {
		name string
		cmp  *ConfigMapPropagation
		want *apis.FieldError
	}{{
		name: "empty configmappropagation spec",
		cmp:  &ConfigMapPropagation{Spec: ConfigMapPropagationSpec{}},
		want: &apis.FieldError{
			Paths:   []string{"spec.originalNamespace, spec.selector"},
			Message: "missing field(s)",
		},
	},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cmp.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("ConfigMapPropagation.Validate (-want, +got) = %v", diff)
			}
		})
	}
}

func TestConfigMapPropagationSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		cmps *ConfigMapPropagationSpec
		want *apis.FieldError
	}{{
		name: "missing configmappropagation spec",
		cmps: &ConfigMapPropagationSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("originalNamespace", "selector")
			return fe
		}(),
	}, {
		name: "missing original namespace",
		cmps: &ConfigMapPropagationSpec{OriginalNamespace: originalNamespace},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("selector")
			return fe
		}(),
	}, {
		name: "missing selector",
		cmps: &ConfigMapPropagationSpec{Selector: validSelector},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("originalNamespace")
			return fe
		}(),
	}, {
		name: "missing selector keys",
		cmps: &ConfigMapPropagationSpec{
			OriginalNamespace: originalNamespace,
			Selector:          map[string]string{}},
		want: &apis.FieldError{
			Message: "At least one selector must be specified",
			Paths:   []string{"selector"},
		},
	}, {
		name: "invalid selector keys, start with number",
		cmps: &ConfigMapPropagationSpec{
			OriginalNamespace: originalNamespace,
			Selector: map[string]string{
				"0nvalid": "testing",
			}},
		want: &apis.FieldError{
			Message: `Invalid selector name: "0nvalid"`,
			Paths:   []string{"selector"},
		},
	}, {
		name: "invalid attribute name, capital letters",
		cmps: &ConfigMapPropagationSpec{
			OriginalNamespace: originalNamespace,
			Selector: map[string]string{
				"inValid": "testing",
			}},
		want: &apis.FieldError{
			Message: `Invalid selector name: "inValid"`,
			Paths:   []string{"selector"},
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cmps.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate ConfigMapPropagationSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestConfigMapPropagationImmutableFields(t *testing.T) {
	tests := []struct {
		name     string
		current  *ConfigMapPropagation
		original *ConfigMapPropagation
		want     *apis.FieldError
	}{{
		name: "good (no change)",
		current: &ConfigMapPropagation{Spec: ConfigMapPropagationSpec{
			OriginalNamespace: currentNamespace,
		}},
		original: &ConfigMapPropagation{Spec: ConfigMapPropagationSpec{
			OriginalNamespace: currentNamespace,
		}},
		want: nil,
	}, {
		name: "new nil is ok",
		current: &ConfigMapPropagation{Spec: ConfigMapPropagationSpec{
			OriginalNamespace: currentNamespace,
		}},
		original: nil,
		want:     nil,
	}, {
		name: "bad (spec change)",
		current: &ConfigMapPropagation{Spec: ConfigMapPropagationSpec{
			OriginalNamespace: currentNamespace,
		}},
		original: &ConfigMapPropagation{Spec: ConfigMapPropagationSpec{
			OriginalNamespace: originalNamespace,
		}},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.ConfigMapPropagationSpec}.OriginalNamespace:
	-: "original"
	+: "current"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotErr := test.current.CheckImmutableFields(context.Background(), test.original)
			if diff := cmp.Diff(test.want.Error(), gotErr.Error()); diff != "" {
				t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}
