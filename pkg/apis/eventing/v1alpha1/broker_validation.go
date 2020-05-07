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

	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

func (b *Broker) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}

func (bs *BrokerSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// As of v0.15 do not allow creation of new Brokers with the channel template.
	if apis.IsInCreate(ctx) {
		if bs.ChannelTemplate != nil {
			errs = errs.Also(apis.ErrDisallowedFields("channelTemplate"))
		}
	} else {
		if bs.ChannelTemplate != nil {
			if cte := messagingv1beta1.IsValidChannelTemplate(bs.ChannelTemplate); cte != nil {
				errs = errs.Also(cte.ViaField("channelTemplateSpec"))
			}
		}
	}

	// TODO validate that the channelTemplate only specifies the channel and arguments.
	return errs
}

func (b *Broker) CheckImmutableFields(ctx context.Context, original *Broker) *apis.FieldError {
	if original == nil {
		return nil
	}

	if diff, err := kmp.ShortDiff(original.Spec.ChannelTemplate, b.Spec.ChannelTemplate); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Broker",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec", "channelTemplate"},
			Details: diff,
		}
	}

	return nil
}
