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
	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
)

// Validate validates the Source resource.
func (s *Source) Validate() *apis.FieldError {
	return s.Spec.Validate().ViaField("spec")
}

// Validate validates the Source spec
func (ss *SourceSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ss, &SourceSpec{}) {
		return apis.ErrMissingField("provisioner")
	}
	var errs *apis.FieldError

	if ss.Channel != nil && !isChannelableEmpty(*ss.Channel) {
		errs = errs.Also(isValidChannelable(*ss.Channel).ViaField("channel"))
	}

	// TODO: could validate that arguments are json if that is a requirement.

	return errs
}
