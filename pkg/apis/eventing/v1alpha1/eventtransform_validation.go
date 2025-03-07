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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

func (t *EventTransform) Validate(ctx context.Context) *apis.FieldError {
	ctx = apis.WithinParent(ctx, t.ObjectMeta)
	return t.Spec.Validate(ctx).ViaField("spec")
}

var possibleTransformations = []string{"jsonata"}

func (ts *EventTransformSpec) Validate(ctx context.Context) *apis.FieldError {
	errs := ts.EventTransformations.Validate(ctx /* allowEmpty */, false)
	errs = errs.Also(ts.Sink.Validate(ctx).ViaField("sink"))
	errs = errs.Also(disallowSinkCaCerts(ts).ViaField("sink"))
	errs = errs.Also(ts.Reply.Validate(ctx, ts).ViaField("reply"))

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*EventTransform)
		errs = errs.Also(ts.CheckImmutableFields(ctx, original))
	}

	return errs
}

func (ets EventTransformations) Validate(ctx context.Context, allowEmpty bool) *apis.FieldError {
	var errs *apis.FieldError

	// Only one type of transformation is allowed.
	// These are transformations field paths.
	transformations := ets.transformations()

	if len(transformations) == 0 && !allowEmpty {
		errs = apis.ErrMissingOneOf(possibleTransformations...)
	} else if len(transformations) > 1 {
		errs = apis.ErrMultipleOneOf(transformations...)
	}

	errs = errs.Also(ets.Jsonata.Validate(ctx).ViaField("jsonata"))

	return errs
}

func (ets EventTransformations) transformations() []string {
	// Only one type of transformation is allowed.
	// These are transformations field paths.
	transformations := make([]string, 0, 2)

	if ets.Jsonata != nil {
		transformations = append(transformations, "jsonata")
	}
	return transformations
}

func (rs *ReplySpec) Validate(ctx context.Context, ts *EventTransformSpec) *apis.FieldError {
	if rs == nil {
		return nil
	}
	if ts.Sink == nil {
		return apis.ErrGeneric(
			"reply is set without spec.sink",
			"",
		)
	}

	errs := rs.EventTransformations.Validate(ctx /* allowEmpty */, true)

	baseTransformationsSet := sets.New(ts.EventTransformations.transformations()...)
	replyTransformationsSet := sets.New(rs.EventTransformations.transformations()...)
	transformationsIntersection := baseTransformationsSet.Intersection(replyTransformationsSet)

	replyTransformations := rs.EventTransformations.transformations()

	if rs.Discard != nil && *rs.Discard {
		replyTransformations = append(replyTransformations, "discard")
	}
	if len(replyTransformations) > 1 {
		errs = apis.ErrMultipleOneOf(replyTransformations...)
	} else if replyTransformationsSet.Len() > 0 &&
		baseTransformationsSet.Len() > 0 &&
		transformationsIntersection.Len() != 1 {
		errs = apis.ErrGeneric(
			fmt.Sprintf(
				"Reply transformation type must match the transformation type in the top-level spec. Top-level transformations: %#v, reply transformations: %#v",
				strings.Join(baseTransformationsSet.UnsortedList(), ", "),
				strings.Join(replyTransformationsSet.UnsortedList(), ", "),
			),
			replyTransformationsSet.UnsortedList()...,
		)
	}

	return errs
}

func (js *JsonataEventTransformationSpec) Validate(context.Context) *apis.FieldError {
	// Jsonata parsers for Go are not maintained, therefore, we will not parse the expression here.
	// The downside is that the errors will only be present in the status of the EventTransform resource.
	// We can reconsider this in the future and improve.
	return nil
}

func disallowSinkCaCerts(ts *EventTransformSpec) *apis.FieldError {
	sink := ts.Sink
	if sink == nil || sink.CACerts == nil || ts.Jsonata == nil {
		return nil
	}
	return &apis.FieldError{
		Message: "CACerts for the sink is not supported for JSONata transformations, to propagate CA trust bundles use labeled ConfigMaps: " +
			"https://knative.dev/docs/eventing/features/transport-encryption/#configure-additional-ca-trust-bundles",
		Paths: []string{"CACerts"},
	}
}

func (in *EventTransformSpec) CheckImmutableFields(ctx context.Context, original *EventTransform) *apis.FieldError {
	if original == nil {
		return nil
	}

	errs := in.EventTransformations.CheckImmutableFields(ctx, original.Spec.EventTransformations)
	errs = errs.Also(in.Reply.CheckImmutableFields(ctx, original.Spec.Reply).ViaField("reply"))
	return errs
}

func (ets EventTransformations) CheckImmutableFields(ctx context.Context, original EventTransformations) *apis.FieldError {
	var errs *apis.FieldError

	const suggestion = "Suggestion: create a new transformation, migrate services to the new one, and delete this transformation."

	if ets.Jsonata != nil && original.Jsonata == nil {
		errs = apis.ErrGeneric("Transformations types are immutable, jsonata transformation cannot be changed to a different transformation type. " + suggestion).ViaField("jsonata")
	} else if original.Jsonata != nil && ets.Jsonata == nil {
		errs = apis.ErrGeneric("Transformations types are immutable, transformation type cannot be changed to a jsonata transformation. " + suggestion).ViaField("jsonata")
	}

	return errs
}

func (rs *ReplySpec) CheckImmutableFields(_ context.Context, _ *ReplySpec) *apis.FieldError {
	// ReplySpec is fully mutable.
	return nil
}
