/*
Copyright 2025 The Knative Authors

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

	"knative.dev/pkg/apis"
)

func (t *EventTransform) Validate(ctx context.Context) *apis.FieldError {
	ctx = apis.WithinParent(ctx, t.ObjectMeta)
	return t.Spec.Validate(ctx).ViaField("spec")
}

var possibleTransformations = []string{"jsonata"}

func (ts *EventTransformSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Only one type of transformation is allowed.
	// These are transformations field paths.
	transformations := make([]string, 0, 2)

	if ts.EventTransformations.Jsonata != nil {
		transformations = append(transformations, "jsonata")
	}

	if len(transformations) == 0 {
		errs = errs.Also(apis.ErrMissingOneOf(possibleTransformations...))
	} else if len(transformations) > 1 {
		errs = errs.Also(apis.ErrMultipleOneOf(transformations...))
	}

	errs = errs.Also(ts.EventTransformations.Jsonata.Validate(ctx).ViaField("jsonata"))
	errs = errs.Also(ts.Sink.Validate(ctx).ViaField("sink"))

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*EventTransform)
		errs = errs.Also(ts.CheckImmutableFields(ctx, original))
	}

	return errs
}

func (js *JsonataEventTransformationSpec) Validate(context.Context) *apis.FieldError {
	// Jsonata parsers for Go are not maintained, therefore, we will not parse the expression here.
	// The downside is that the errors will only be present in the status of the EventTransform resource.
	// We can reconsider this in the future and improve.
	return nil
}

func (in *EventTransformSpec) CheckImmutableFields(_ context.Context, original *EventTransform) *apis.FieldError {
	if original == nil {
		return nil
	}

	var errs *apis.FieldError

	if original.Spec.EventTransformations.Jsonata != nil && in.EventTransformations.Jsonata == nil {
		errs = errs.Also(apis.ErrGeneric("transformations types are immutable, jsonata transformation cannot be changed to a different transformation type"))
	}

	return errs
}
