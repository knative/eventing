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
	"fmt"

	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/equality"
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

	missingSubscriber := isDestinationNilOrEmpty(ss.Subscriber)
	missingReply := isDestinationNilOrEmpty(ss.Reply)
	if missingSubscriber && missingReply {
		fe := apis.ErrMissingField("reply", "subscriber")
		fe.Details = "the Subscription must reference at least one of (reply or a subscriber)"
		errs = errs.Also(fe)
	}

	if !missingSubscriber {
		if true /* experimental feature enable */ {
			// Special validation to enable the experimental feature.
			// This logic is going to be moved back to Destination.Validate once the experimentation is done
			if fe := validateDestinationWithGroupExperimentalFeatureEnabled(ctx, ss.Subscriber); fe != nil {
				errs = errs.Also(fe.ViaField("subscriber"))
			}
		} else {
			if fe := ss.Subscriber.Validate(ctx); fe != nil {
				errs = errs.Also(fe.ViaField("subscriber"))
			}
		}
	}

	if !missingReply {
		if fe := ss.Reply.Validate(ctx); fe != nil {
			errs = errs.Also(fe.ViaField("reply"))
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
	ignoreArguments := cmpopts.IgnoreFields(SubscriptionSpec{}, "Subscriber", "Reply")
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

func validateDestinationWithGroupExperimentalFeatureEnabled(ctx context.Context, dest *duckv1.Destination) *apis.FieldError {
	ref := dest.Ref
	uri := dest.URI
	if ref == nil && uri == nil {
		return apis.ErrGeneric("expected at least one, got none", "ref", "uri")
	}

	if ref != nil && uri != nil && uri.URL().IsAbs() {
		return apis.ErrGeneric("Absolute URI is not allowed when Ref or [apiVersion, kind, name] is present", "[apiVersion, kind, name]", "ref", "uri")
	}
	// IsAbs() check whether the URL has a non-empty scheme. Besides the non-empty scheme, we also require uri has a non-empty host
	if ref == nil && uri != nil && (!uri.URL().IsAbs() || uri.Host == "") {
		return apis.ErrInvalidValue("Relative URI is not allowed when Ref and [apiVersion, kind, name] is absent", "uri")
	}
	if ref != nil && uri == nil {
		return ref.Validate(ctx).ViaField("ref")
	}
	return nil
}

func validateKReferenceWithGroupExperimentalFeatureEnabled(ctx context.Context, kr *duckv1.KReference) *apis.FieldError {
	var errs *apis.FieldError
	if kr == nil {
		return errs.Also(apis.ErrMissingField("name")).
			Also(apis.ErrMissingField("apiVersion")).
			Also(apis.ErrMissingField("kind"))
	}
	if kr.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if kr.APIVersion == "" && kr.Group == "" {
		errs = errs.Also(apis.ErrMissingField("apiVersion")).
			Also(apis.ErrMissingField("group"))
	}
	if kr.APIVersion != "" && kr.Group != "" {
		errs = errs.Also(&apis.FieldError{
			Message: "both apiVersion and group are specified",
			Paths:   []string{"apiVersion", "group"},
			Details: "Only one of them must be specified",
		})
	}
	if kr.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind"))
	}
	// Only if namespace is empty validate it. This is to deal with legacy
	// objects in the storage that may now have the namespace filled in.
	// Because things get defaulted in other cases, moving forward the
	// kr.Namespace will not be empty.
	if kr.Namespace != "" {
		if !apis.IsDifferentNamespaceAllowed(ctx) {
			parentNS := apis.ParentMeta(ctx).Namespace
			if parentNS != "" && kr.Namespace != parentNS {
				errs = errs.Also(&apis.FieldError{
					Message: "mismatched namespaces",
					Paths:   []string{"namespace"},
					Details: fmt.Sprintf("parent namespace: %q does not match ref: %q", parentNS, kr.Namespace),
				})
			}
		}
	}

	return errs
}
