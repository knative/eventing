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

// We require always Channel
// Also at least one of 'subscriber' and 'reply' must be defined (non-nill and non-empty)
func (ss *SubscriptionSpec) Validate() *apis.FieldError {
	var errs *apis.FieldError
	if isChannelEmpty(ss.Channel) {
		fe := apis.ErrMissingField("channel")
		fe.Details = "the Subscription must reference a channel"
		return fe
	} else if fe := isValidChannel(ss.Channel); fe != nil {
		errs = errs.Also(fe.ViaField("channel"))
	}

	missingSubscriber := isSubscriberSpecNilOrEmpty(ss.Subscriber)
	missingReply := isReplyStrategyNilOrEmpty(ss.Reply)
	if missingSubscriber && missingReply {
		fe := apis.ErrMissingField("reply", "subscriber")
		fe.Details = "the Subscription must reference at least one of (reply or a subscriber)"
		errs = errs.Also(fe)
	}

	if !missingSubscriber {
		if fe := isValidSubscriberSpec(*ss.Subscriber); fe != nil {
			errs = errs.Also(fe.ViaField("subscriber"))
		}
	}

	if !missingReply {
		if fe := isValidReply(*ss.Reply); fe != nil {
			errs = errs.Also(fe.ViaField("reply"))
		}
	}

	return errs
}

func isSubscriberSpecNilOrEmpty(s *SubscriberSpec) bool {
	return s == nil || equality.Semantic.DeepEqual(s, &SubscriberSpec{}) ||
		(equality.Semantic.DeepEqual(s.Ref, &corev1.ObjectReference{}) && s.DNSName == nil)

}

func isValidSubscriberSpec(s SubscriberSpec) *apis.FieldError {
	var errs *apis.FieldError
	if s.DNSName != nil && *s.DNSName != "" && s.Ref != nil && !equality.Semantic.DeepEqual(s.Ref, &corev1.ObjectReference{}) {
		errs = errs.Also(apis.ErrMultipleOneOf("ref", "dnsName"))
	}

	// If Ref given, check the fields.
	if s.Ref != nil && !equality.Semantic.DeepEqual(s.Ref, &corev1.ObjectReference{}) {
		fe := isValidObjectReference(*s.Ref)
		if fe != nil {
			errs = errs.Also(fe.ViaField("ref"))
		}
	}
	return errs
}

func isReplyStrategyNilOrEmpty(r *ReplyStrategy) bool {
	return r == nil || equality.Semantic.DeepEqual(r, &ReplyStrategy{}) || equality.Semantic.DeepEqual(r.Channel, &corev1.ObjectReference{})
}

func isValidReply(r ReplyStrategy) *apis.FieldError {
	if fe := isValidObjectReference(*r.Channel); fe != nil {
		return fe.ViaField("channel")
	}
	if r.Channel.Kind != "Channel" {
		fe := apis.ErrInvalidValue(r.Channel.Kind, "kind")
		fe.Paths = []string{"kind"}
		fe.Details = "only 'Channel' kind is allowed"
		return fe
	}
	if r.Channel.APIVersion != "eventing.knative.dev/v1alpha1" {
		fe := apis.ErrInvalidValue(r.Channel.APIVersion, "apiVersion")
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

	// Only Subscriber and Reply are mutable.
	ignoreArguments := cmpopts.IgnoreFields(SubscriptionSpec{}, "Subscriber", "Reply")
	if diff := cmp.Diff(original.Spec, current.Spec, ignoreArguments); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
