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

	"github.com/google/go-cmp/cmp/cmpopts"
	"knative.dev/pkg/kmp"

	"knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

func (c *Channel) Validate(ctx context.Context) *apis.FieldError {
	return c.Spec.Validate(ctx).ViaField("spec")
}

func (cs *ChannelSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if cs.ChannelTemplate == nil {
		// The Channel defaulter is expected to set this, not the users.
		errs = errs.Also(apis.ErrMissingField("channelTemplate"))
	} else if cte := v1beta1.IsValidChannelTemplate(cs.ChannelTemplate); cte != nil {
		errs = errs.Also(cte.ViaField("channelTemplate"))
	}

	if cs.Subscribable != nil && len(cs.Subscribable.Subscribers) > 0 {
		errs = errs.Also(apis.ErrDisallowedFields("subscribers").ViaField("subscribable"))
	}

	return errs
}

func (c *Channel) CheckImmutableFields(ctx context.Context, original *Channel) *apis.FieldError {
	if original == nil {
		return nil
	}

	ignoreArguments := cmpopts.IgnoreFields(ChannelSpec{}, "Subscribable")
	if diff, err := kmp.ShortDiff(original.Spec, c.Spec, ignoreArguments); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Channel",
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
