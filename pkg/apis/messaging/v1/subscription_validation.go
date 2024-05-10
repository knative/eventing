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

package v1

import (
	"context"

	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/eventing/pkg/apis/feature"
	cn "knative.dev/eventing/pkg/crossnamespace"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmp"
)

func (s *Subscription) Validate(ctx context.Context) *apis.FieldError {
	errs := s.Spec.Validate(ctx).ViaField("spec")
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Subscription)
		errs = errs.Also(s.CheckImmutableFields(ctx, original))
	}
	// s.Validate(ctx) because krshaped is defined on the entire subscription, not just the spec
	if feature.FromContext(ctx).IsEnabled(feature.CrossNamespaceEventLinks) {
		crossNamespaceError := cn.CheckNamespace(ctx, s)
		if crossNamespaceError != nil {
			errs = errs.Also(crossNamespaceError)
		}
	}
	return errs
}

func (ss *SubscriptionSpec) Validate(ctx context.Context) *apis.FieldError {
	// We require always Channel.
	// Also at least one of 'subscriber' and 'reply' must be defined (non-nil and non-empty).

	var errs *apis.FieldError
	if isChannelEmpty(ss.Channel) {
		fe := apis.ErrMissingField("channel")
		fe.Details = "the Subscription must reference a channel"
		return fe
	} else if fe := isValidChannel(ctx, ss.Channel); fe != nil {
		errs = errs.Also(fe.ViaField("channel"))
	}

	// Check if we follow the spec and have a valid reference to a subscriber
	if isDestinationNilOrEmpty(ss.Subscriber) {
		fe := apis.ErrMissingField("subscriber")
		fe.Details = "the Subscription must reference a subscriber"
		errs = errs.Also(fe)
	} else {
		if fe := ss.Subscriber.Validate(ctx); fe != nil {
			errs = errs.Also(fe.ViaField("subscriber"))
		}
	}

	if !isDestinationNilOrEmpty(ss.Reply) {
		if fe := ss.Reply.Validate(ctx); fe != nil {
			errs = errs.Also(fe.ViaField("reply"))
		}
	}

	if ss.Delivery != nil {
		if fe := ss.Delivery.Validate(ctx); fe != nil {
			errs = errs.Also(fe.ViaField("delivery"))
		}
	}

	return errs
}

func isDestinationNilOrEmpty(d *duckv1.Destination) bool {
	return d == nil || equality.Semantic.DeepEqual(d, &duckv1.Destination{})
}

func (s *Subscription) CheckImmutableFields(ctx context.Context, original *Subscription) *apis.FieldError {
	if original == nil {
		return nil
	}

	// Only Subscriber and Reply are mutable.
	ignoreArguments := cmpopts.IgnoreFields(SubscriptionSpec{}, "Subscriber", "Reply", "Delivery")
	if diff, err := kmp.ShortDiff(original.Spec, s.Spec, ignoreArguments); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Subscription",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
