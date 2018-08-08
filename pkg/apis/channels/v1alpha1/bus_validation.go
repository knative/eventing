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
	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/util/validation"
)

// TODO(n3wscott): This is staging work, the plan is another pass to bring up
// the test coverage, then remove unused after each type is stubbed.
// This is all prep for new serving style webhook integration.

func (b *Bus) Validate() *apis.FieldError {
	return b.Spec.Validate().ViaField("spec")
}

func (bs *BusSpec) Validate() *apis.FieldError {
	if bs.Parameters != nil {
		return bs.Parameters.Validate().ViaField("parameters")
	}
	return nil
}

func (bp *BusParameters) Validate() *apis.FieldError {
	if bp.Channel != nil {
		for i, p := range *bp.Channel {
			errs := validation.IsConfigMapKey(p.Name)
			if len(errs) > 0 {
				return apis.ErrInvalidKeyName(p.Name, "name", errs...).ViaField(fmt.Sprintf("channel[%d]", i))
			}
		}
	}
	if bp.Subscription != nil {
		for i, p := range *bp.Subscription {
			errs := validation.IsConfigMapKey(p.Name)
			if len(errs) > 0 {
				return apis.ErrInvalidKeyName(p.Name, "name", errs...).ViaField(fmt.Sprintf("subscription[%d]", i))
			}
		}
	}
	return nil
}

func (current *Bus) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	// TODO(n3wscott): Anything to check?
	return nil
}
