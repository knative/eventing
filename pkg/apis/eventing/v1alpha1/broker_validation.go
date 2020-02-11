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

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

func (b *Broker) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}

func (bs *BrokerSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Validate the new channelTemplate.
	// TODO: As part of https://github.com/knative/eventing/issues/2128
	// Also make sure this gets rejected. It would break our tests
	// and assumptions to do this right now.
	//	if bs.ChannelTemplate == nil {
	//		errs = errs.Also(apis.ErrMissingField("channelTemplateSpec"))
	//	} else
	if cte := isValidChannelTemplate(bs.ChannelTemplate); cte != nil {
		errs = errs.Also(cte.ViaField("channelTemplateSpec"))
	}

	// TODO validate that the channelTemplate only specifies the channel and arguments.
	return errs
}

func isValidChannelTemplate(dct *eventingduckv1alpha1.ChannelTemplateSpec) *apis.FieldError {
	if dct == nil {
		return nil
	}
	var errs *apis.FieldError
	if dct.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind"))
	}
	if dct.APIVersion == "" {
		errs = errs.Also(apis.ErrMissingField("apiVersion"))
	}
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
