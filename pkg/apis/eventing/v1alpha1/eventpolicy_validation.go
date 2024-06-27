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

package v1alpha1

import (
	"context"
	"strings"

	"knative.dev/pkg/apis"
)

func (ep *EventPolicy) Validate(ctx context.Context) *apis.FieldError {
	return ep.Spec.Validate(ctx).ViaField("spec")
}

func (ets *EventPolicySpec) Validate(ctx context.Context) *apis.FieldError {
	var err *apis.FieldError
	for i, f := range ets.From {
		if f.Ref == nil && (f.Sub == nil || *f.Sub == "") {
			err = err.Also(apis.ErrMissingOneOf("ref", "sub").ViaFieldIndex("from", i))
		}
		if f.Ref != nil && f.Sub != nil {
			err = err.Also(apis.ErrMultipleOneOf("ref", "sub").ViaFieldIndex("from", i))
		}
		err = err.Also(f.Ref.Validate().ViaField("ref").ViaFieldIndex("from", i))
		err = err.Also(validateSub(f.Sub).ViaField("sub").ViaFieldIndex("from", i))
	}

	for i, t := range ets.To {
		if t.Ref == nil && t.Selector == nil {
			err = err.Also(apis.ErrMissingOneOf("ref", "selector").ViaFieldIndex("to", i))
		}
		if t.Ref != nil && t.Selector != nil {
			err = err.Also(apis.ErrMultipleOneOf("ref", "selector").ViaFieldIndex("to", i))
		}
		if t.Ref != nil {
			err = err.Also(t.Ref.Validate().ViaField("ref").ViaFieldIndex("to", i))
		}
	}

	return err
}

func validateSub(sub *string) *apis.FieldError {
	if sub == nil || len(*sub) <= 1 {
		return nil
	}

	lastInvalidIdx := len(*sub) - 2
	firstInvalidIdx := 0
	if idx := strings.IndexRune(*sub, '*'); idx >= firstInvalidIdx && idx <= lastInvalidIdx {
		return apis.ErrInvalidValue(*sub, "", "'*' is only allowed as suffix")
	}

	return nil
}

func (r *EventPolicyFromReference) Validate() *apis.FieldError {
	if r == nil {
		return nil
	}

	var err *apis.FieldError
	if r.Kind == "" {
		err = err.Also(apis.ErrMissingField("kind"))
	}
	if r.APIVersion == "" {
		err = err.Also(apis.ErrMissingField("apiVersion"))
	}
	if r.Name == "" {
		err = err.Also(apis.ErrMissingField("name"))
	}
	return err
}

func (r *EventPolicyToReference) Validate() *apis.FieldError {
	var err *apis.FieldError
	if r.Kind == "" {
		err = err.Also(apis.ErrMissingField("kind"))
	}
	if r.APIVersion == "" {
		err = err.Also(apis.ErrMissingField("apiVersion"))
	}
	if r.Name == "" {
		err = err.Also(apis.ErrMissingField("name"))
	}
	return err
}
