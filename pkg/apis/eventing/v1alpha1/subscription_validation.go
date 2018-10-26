/*
Copyright 2018 The Knative Authors

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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (s *Subscription) Validate() *apis.FieldError {
	return s.Spec.Validate().ViaField("spec")
}

// We require always From
// Also at least one of 'call' and 'result' must be defined (non-nill and non-empty)
func (ss *SubscriptionSpec) Validate() *apis.FieldError {
	var errs *apis.FieldError
	if isFromEmpty(ss.From) {
		fe := apis.ErrMissingField("from")
		fe.Details = "the Subscription must reference a from channel"
		return fe
	} else if fe := isValidFrom(ss.From); fe != nil {
		errs = errs.Also(fe.ViaField("from"))
	}

	missingCall := isEndpointSpecNilOrEmpty(ss.Call)
	missingResultStrategy := isResultStrategyNilOrEmpty(ss.Result)
	if missingCall && missingResultStrategy {
		fe := apis.ErrMissingField("result", "call")
		fe.Details = "the Subscription must reference at least one of (result channel or a call)"
		errs = errs.Also(fe)
	}

	if !missingCall {
		if fe := isValidEndpointSpec(*ss.Call); fe != nil {
			errs = errs.Also(fe.ViaField("call"))
		}
	}

	if !missingResultStrategy {
		if fe := isValidResultStrategy(*ss.Result); fe != nil {
			errs = errs.Also(fe.ViaField("result"))
		}
	}

	return errs
}

func isEndpointSpecNilOrEmpty(e *EndpointSpec) bool {
	return e == nil || equality.Semantic.DeepEqual(e, &EndpointSpec{}) ||
		(equality.Semantic.DeepEqual(e.TargetRef, &corev1.ObjectReference{}) && e.DNSName == nil)

}

func isValidEndpointSpec(e EndpointSpec) *apis.FieldError {
	var errs *apis.FieldError
	if e.DNSName != nil && *e.DNSName != "" && e.TargetRef != nil && !equality.Semantic.DeepEqual(e.TargetRef, &corev1.ObjectReference{}) {
		errs = errs.Also(apis.ErrMultipleOneOf("targetRef", "dnsName"))
	}

	// If TargetRef given, check the fields.
	if e.TargetRef != nil && !equality.Semantic.DeepEqual(e.TargetRef, &corev1.ObjectReference{}) {
		fe := isValidObjectReference(*e.TargetRef)
		if fe != nil {
			errs = errs.Also(fe.ViaField("targetRef"))
		}
	}
	return errs
}

func isFromEmpty(f corev1.ObjectReference) bool {
	return isChannelEmpty(f)
}

// Valid from only contains the following fields:
// - Kind       == 'Channel'
// - APIVersion == 'eventing.knative.dev/v1alpha1'
// - Name       == not empty
func isValidFrom(f corev1.ObjectReference) *apis.FieldError {
	return isValidChannel(f)
}

func isResultStrategyNilOrEmpty(r *ResultStrategy) bool {
	return r == nil || equality.Semantic.DeepEqual(r, &ResultStrategy{}) || equality.Semantic.DeepEqual(r.Target, &corev1.ObjectReference{})
}

func isValidResultStrategy(r ResultStrategy) *apis.FieldError {
	fe := isValidObjectReference(*r.Target)
	if fe != nil {
		return fe.ViaField("target")
	}
	if r.Target.Kind != "Channel" {
		fe := apis.ErrInvalidValue(r.Target.Kind, "kind")
		fe.Paths = []string{"kind"}
		fe.Details = "only 'Channel' kind is allowed"
		return fe
	}
	if r.Target.APIVersion != "eventing.knative.dev/v1alpha1" {
		fe := apis.ErrInvalidValue(r.Target.APIVersion, "apiVersion")
		fe.Details = "only eventing.knative.dev/v1alpha1 is allowed for apiVersion"
		return fe
	}
	return nil
}

func (current *Subscription) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	original, ok := og.(*Subscription)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a Subscription"}
	}
	if original == nil {
		return nil
	}

	// Only Call and Result are mutable.
	ignoreArguments := cmpopts.IgnoreFields(SubscriptionSpec{}, "Call", "Result")
	if diff := cmp.Diff(original.Spec, current.Spec, ignoreArguments); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
