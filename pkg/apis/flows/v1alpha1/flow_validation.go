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

func (f *Flow) Validate() *apis.FieldError {
	return f.Spec.Validate().ViaField("spec")
}

func (fs *FlowSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(fs, &FlowSpec{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}

	if err := fs.Trigger.Validate(); err != nil {
		return err.ViaField("trigger")
	}
	if err := fs.Action.Validate(); err != nil {
		return err.ViaField("action")
	}
	return nil
}

func (et *EventTrigger) Validate() *apis.FieldError {
	// TODO(n3wscott): Implement this.
	return nil
}

func (fa *FlowAction) Validate() *apis.FieldError {
	// TODO(n3wscott): Implement this.
	return nil
}

func (current *Flow) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	// TODO(n3wscott): Anything Immutable?
	return nil
}
