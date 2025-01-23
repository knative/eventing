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

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/apis/eventing"
)

var eventingControllerSAName string

func init() {
	eventingControllerSAName = fmt.Sprintf("system:serviceaccount:%s:eventing-controller", system.Namespace())
}

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

	if apis.IsInUpdate(ctx) {
		// Validate that if any changes were made to spec.subscribers, they were made by the eventing-controller
		original := apis.GetBaseline(ctx).(*InMemoryChannel)
		errs = errs.Also(imc.CheckSubscribersChangeAllowed(ctx, original))
	}

	return errs
}

func (imcs *InMemoryChannelSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	for i, subscriber := range imcs.SubscribableSpec.Subscribers {
		if subscriber.ReplyURI == nil && subscriber.SubscriberURI == nil {
			fe := apis.ErrMissingField("replyURI", "subscriberURI")
			fe.Details = "expected at least one of, got none"
			errs = errs.Also(fe.ViaField(fmt.Sprintf("subscriber[%d]", i)).ViaField("subscribable"))
		}
	}

	return errs
}

func (imc *InMemoryChannel) CheckSubscribersChangeAllowed(ctx context.Context, original *InMemoryChannel) *apis.FieldError {
	if original == nil {
		return nil
	}

	if !canChangeChannelSpecAuth(ctx) {
		return imc.checkSubsciberSpecAuthChanged(original, ctx)
	}
	return nil
}

func (imc *InMemoryChannel) checkSubsciberSpecAuthChanged(original *InMemoryChannel, ctx context.Context) *apis.FieldError {
	if diff, err := kmp.ShortDiff(original.Spec.Subscribers, imc.Spec.Subscribers); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Channel.Spec.Subscribers",
			Paths:   []string{"spec.subscribers"},
			Details: err.Error(),
		}
	} else if diff != "" {
		user := apis.GetUserInfo(ctx)
		userName := ""
		if user != nil {
			userName = user.Username
		}
		return &apis.FieldError{
			Message: fmt.Sprintf("Channel.Spec.Subscribers changed by user %s which was not the %s service account", userName, eventingControllerSAName),
			Paths:   []string{"spec.subscribers"},
			Details: diff,
		}
	}
	return nil
}

func canChangeChannelSpecAuth(ctx context.Context) bool {
	user := apis.GetUserInfo(ctx)
	if user == nil {
		return false
	}
	return user.Username == eventingControllerSAName
}
