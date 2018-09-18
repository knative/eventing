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
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
)

func (c *Channel) Validate() *apis.FieldError {
	return c.Spec.Validate().ViaField("spec")
}

func (cs *ChannelSpec) Validate() *apis.FieldError {
	if cs.Provisioner == nil {
		return apis.ErrMissingField("provisioner")
	}

	for i, subscriber := range cs.Subscribers {
		if subscriber.Call == nil && subscriber.Result == nil {
			//TODO collect all errors instead of returning the first. This isn't
			// possible yet with knative/pkg validation.
			return apis.ErrMissingField("call", "result").ViaField(fmt.Sprintf("subscriber[%d]", i))
		}
	}

	return nil
}

func (current *Channel) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	if og == nil {
		return nil
	}

	original, ok := og.(*Channel)
	if !ok {
		return &apis.FieldError{Message: "The provided resource was not a Channel"}
	}
	ignoreArguments := cmpopts.IgnoreFields(ChannelSpec{}, "Arguments")
	if diff := cmp.Diff(original.Spec, current.Spec, ignoreArguments); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed",
			Paths:   []string{"spec.provisioner"},
		}
	}
	return nil
}
