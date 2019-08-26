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
	"fmt"
	"regexp"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"

	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
)

var (
	// Only allow lowercase alphanumeric, starting with letters.
	validAttributeName = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
)

// Validate the Trigger.
func (t *Trigger) Validate(ctx context.Context) *apis.FieldError {
	return t.Spec.Validate(ctx).ViaField("spec")
}

// Validate the TriggerSpec.
func (ts *TriggerSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if ts.Broker == "" {
		fe := apis.ErrMissingField("broker")
		errs = errs.Also(fe)
	}

	if ts.Filter == nil {
		fe := apis.ErrMissingField("filter")
		errs = errs.Also(fe)
	}

	if ts.Filter != nil {
		filtersSpecified := make([]string, 0)

		if ts.Filter.DeprecatedSourceAndType != nil {
			filtersSpecified = append(filtersSpecified, "filter.sourceAndType")
		}

		if ts.Filter.Attributes != nil {
			filtersSpecified = append(filtersSpecified, "filter.attributes")
			if len(*ts.Filter.Attributes) == 0 {
				fe := &apis.FieldError{
					Message: "At least one filtered attribute must be specified",
					Paths:   []string{"filter.attributes"},
				}
				errs = errs.Also(fe)
			} else {
				attrs := map[string]string(*ts.Filter.Attributes)
				for attr := range attrs {
					if !validAttributeName.MatchString(attr) {
						fe := &apis.FieldError{
							Message: fmt.Sprintf("Invalid attribute name: %s", attr),
							Paths:   []string{"filter.attributes"},
						}
						errs = errs.Also(fe)
					}
				}
			}
		}

		if len(filtersSpecified) > 1 {
			fe := apis.ErrMultipleOneOf(filtersSpecified...)
			errs = errs.Also(fe)
		}
	}

	if messagingv1alpha1.IsSubscriberSpecNilOrEmpty(ts.Subscriber) {
		fe := apis.ErrMissingField("subscriber")
		errs = errs.Also(fe)
	} else if fe := messagingv1alpha1.IsValidSubscriberSpec(*ts.Subscriber); fe != nil {
		errs = errs.Also(fe.ViaField("subscriber"))
	}

	return errs
}

// CheckImmutableFields checks that any immutable fields were not changed.
func (t *Trigger) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	if og == nil {
		return nil
	}

	original, ok := og.(*Trigger)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a Trigger"}
	}

	if diff, err := kmp.ShortDiff(original.Spec.Broker, t.Spec.Broker); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Trigger",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec", "broker"},
			Details: diff,
		}
	}
	return nil
}
