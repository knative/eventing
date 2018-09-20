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
	"reflect"
	//	"strings"

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

	missingCallable := isCallableNilOrEmpty(ss.Call)
	missingResultStrategy := isResultStrategyNilOrEmpty(ss.Result)
	if missingCallable && missingResultStrategy {
		fe := apis.ErrMissingField("result", "call")
		fe.Details = "the Subscription must reference at least one of (result channel or a call)"
		errs = errs.Also(fe)
	}

	if !missingCallable {
		if fe := isValidCallable(*ss.Call); fe != nil {
			errs = errs.Also(fe.ViaField("call"))
		}
	}

	if !missingResultStrategy {
		if fe := isValidResultStrategy(ss.Result); fe != nil {
			errs = errs.Also(fe.ViaField("result"))
		}
	}

	return errs
}

func isCallableNilOrEmpty(c *Callable) bool {
	return c == nil || equality.Semantic.DeepEqual(c, &Callable{}) ||
		(equality.Semantic.DeepEqual(c.Target, &corev1.ObjectReference{}) && c.TargetURI == nil)

}

func isValidCallable(c Callable) *apis.FieldError {
	var errs *apis.FieldError
	if c.TargetURI != nil && *c.TargetURI != "" && c.Target != nil && !equality.Semantic.DeepEqual(c.Target, &corev1.ObjectReference{}) {
		errs = errs.Also(apis.ErrMultipleOneOf("target", "targetURI"))
	}

	// If Target given, check the fields.
	if c.Target != nil && !equality.Semantic.DeepEqual(c.Target, &corev1.ObjectReference{}) {
		fe := isValidObjectReference(*c.Target)
		if fe != nil {
			errs = errs.Also(fe.ViaField("target"))
		}
	}
	return errs
}

func isFromEmpty(f corev1.ObjectReference) bool {
	return equality.Semantic.DeepEqual(f, corev1.ObjectReference{})
}

// Valid from only contains the following fields:
// - Kind       == 'Channel'
// - APIVersion == 'channels.knative.dev/v1alpha1'
// - Name       == not empty
func isValidFrom(f corev1.ObjectReference) *apis.FieldError {
	errs := isValidObjectReference(f)

	if f.Kind != "Channel" {
		fe := apis.ErrInvalidValue(f.Kind, "kind")
		fe.Paths = []string{"kind"}
		fe.Details = "only 'Channel' kind is allowed"
		errs = errs.Also(fe)
	}
	if f.APIVersion != "channels.knative.dev/v1alpha1" {
		fe := apis.ErrInvalidValue(f.APIVersion, "apiVersion")
		fe.Details = "only channels.knative.dev/v1alpha1 is allowed for apiVersion"
		errs = errs.Also(fe)
	}
	return errs
}

func isResultStrategyNilOrEmpty(r *ResultStrategy) bool {
	return r == nil || equality.Semantic.DeepEqual(r, &ResultStrategy{}) || equality.Semantic.DeepEqual(r.Target, &corev1.ObjectReference{})
}

func isValidResultStrategy(r *ResultStrategy) *apis.FieldError {
	return isValidObjectReference(*r.Target).ViaField("target")
}

func isValidObjectReference(f corev1.ObjectReference) *apis.FieldError {
	return checkRequiredFields(f).
		Also(checkDisallowedFields(f))
}

// Check the corev1.ObjectReference to make sure it has the required fields. They
// are not checked for anything more except that they are set.
func checkRequiredFields(f corev1.ObjectReference) *apis.FieldError {
	var errs *apis.FieldError
	if f.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if f.APIVersion == "" {
		errs = errs.Also(apis.ErrMissingField("apiVersion"))
	}
	if f.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind"))
	}
	return errs
}

// Check the corev1.ObjectReference to make sure it only has the following fields set:
// Name, Kind, APIVersion
// If any other fields are set and is not the Zero value, returns an apis.FieldError
// with the fieldpaths for all those fields.
func checkDisallowedFields(f corev1.ObjectReference) *apis.FieldError {
	disallowedFields := []string{}
	// See if there are any fields that have been set that should not be.
	// TODO: Hoist this kind of stuff into pkg repository.
	s := reflect.ValueOf(f)
	typeOf := s.Type()
	for i := 0; i < s.NumField(); i++ {
		field := s.Field(i)
		fieldName := typeOf.Field(i).Name
		if fieldName == "Name" || fieldName == "Kind" || fieldName == "APIVersion" {
			continue
		}
		if !cmp.Equal(field.Interface(), reflect.Zero(field.Type()).Interface()) {
			disallowedFields = append(disallowedFields, fieldName)
		}
	}
	if len(disallowedFields) > 0 {
		fe := apis.ErrDisallowedFields(disallowedFields...)
		fe.Details = "only name, apiVersion and kind are supported fields"
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
