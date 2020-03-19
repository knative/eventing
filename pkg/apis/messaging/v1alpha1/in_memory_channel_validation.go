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
	"fmt"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/validation"
	"knative.dev/pkg/apis"
)

func (imc *InMemoryChannel) Validate(ctx context.Context) *apis.FieldError {
	errs := imc.Spec.Validate(ctx).ViaField("spec")

	// Validate annotations
	if imc.Annotations != nil {
		if scope, ok := imc.Annotations[eventing.ScopeAnnotationKey]; ok {
			if scope != eventing.ScopeNamespace && scope != eventing.ScopeCluster {
				iv := apis.ErrInvalidValue(scope, "")
				iv.Details = "expected either 'cluster' or 'namespace'"
				errs = errs.Also(iv.ViaFieldKey("annotations", eventing.ScopeAnnotationKey).ViaField("metadata"))
			}
		}
	}

	if err := imc.checkImmutableFields(ctx); err != nil {
		errs = errs.Also(err)
	}

	return errs
}

func (imcs *InMemoryChannelSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if imcs.Subscribable != nil {
		for i, subscriber := range imcs.Subscribable.Subscribers {
			if subscriber.ReplyURI == nil && subscriber.SubscriberURI == nil {
				fe := apis.ErrMissingField("replyURI", "subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe.ViaField(fmt.Sprintf("subscriber[%d]", i)).ViaField("subscribable"))
			}
		}
	}

	return errs
}

func (imcs *InMemoryChannel) checkImmutableFields(ctx context.Context) *apis.FieldError {

	if !apis.IsInUpdate(ctx) {
		return nil
	}

	original := apis.GetBaseline(ctx).(*InMemoryChannel)

	if err := validation.Scope(original.GetAnnotations(), imcs.GetAnnotations()); err != nil {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid %s annotation value", eventing.ScopeAnnotationKey),
			Paths:   []string{"metadata.annotations"},
			Details: err.Error(),
		}
	}

	return nil
}