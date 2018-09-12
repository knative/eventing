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

func (ss *SubscriptionSpec) Validate() *apis.FieldError {
	if ss.From == nil || equality.Semantic.DeepEqual(ss.From, &corev1.ObjectReference{}) {
		fe := apis.ErrMissingField("from")
		fe.Details = "the Subscription must reference a from channel"
		return fe
	}
	if ss.Processor == nil && ss.To == nil {
		fe := apis.ErrMissingField("to", "processor")
		fe.Details = "the Subscription must reference at least one of (to channel or a processor)"
		return fe
	}

	if equality.Semantic.DeepEqual(ss.Processor, &corev1.ObjectReference{}) && equality.Semantic.DeepEqual(ss.To, &corev1.ObjectReference{}) {
		fe := apis.ErrMissingField("to", "processor")
		fe.Details = "the Subscription must reference at least one of (to channel or a processor)"
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

	ignoreArguments := cmpopts.IgnoreFields(SubscriptionSpec{}, "Processor", "Arguments")
	if diff := cmp.Diff(original.Spec, current.Spec, ignoreArguments); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
