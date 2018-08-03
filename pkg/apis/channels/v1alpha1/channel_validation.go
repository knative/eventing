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

	"k8s.io/apimachinery/pkg/util/validation"
)

func (c *Channel) Validate() *apis.FieldError {
	return c.Spec.Validate().ViaField("spec")
}

func (cs *ChannelSpec) Validate() *apis.FieldError {
	switch {
	case len(cs.Bus) != 0 && len(cs.ClusterBus) != 0:
		return apis.ErrMultipleOneOf("bus", "clusterBus")
	case len(cs.Bus) != 0:
		if errs := validation.IsQualifiedName(cs.Bus); len(errs) > 0 {
			return apis.ErrInvalidKeyName(cs.Bus, "bus", errs...)
		}
		return nil
	case len(cs.ClusterBus) != 0:
		if errs := validation.IsQualifiedName(cs.ClusterBus); len(errs) > 0 {
			return apis.ErrInvalidKeyName(cs.ClusterBus, "clusterBus", errs...)
		}
		return nil
	default:
		return apis.ErrMissingOneOf("bus", "clusterBus")
	}
}

func (current *Channel) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	original, ok := og.(*Channel)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a Channel"}
	}
	if original == nil {
		return nil
	}

	ignoreArguments := cmpopts.IgnoreFields(ChannelSpec{}, "Arguments")
	if diff := cmp.Diff(original.Spec, current.Spec, ignoreArguments); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
