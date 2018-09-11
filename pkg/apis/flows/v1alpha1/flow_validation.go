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
	"k8s.io/apimachinery/pkg/util/validation"
	"net/url"
)

// Validate validates the Flow resource.
func (f *Flow) Validate() *apis.FieldError {
	return f.Spec.Validate().ViaField("spec")
}

// Validate validates the flow spec for a valid action and trigger.
func (fs *FlowSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(fs, &FlowSpec{}) {
		return apis.ErrMissingField("trigger", "action")
	}

	if err := fs.Trigger.Validate(); err != nil {
		return err.ViaField("trigger")
	}
	if err := fs.Action.Validate(); err != nil {
		return err.ViaField("action")
	}
	return nil
}

// Validate validates event trigger has both eventType and resource set.
func (et *EventTrigger) Validate() *apis.FieldError {

	if et.EventType == "" {
		return apis.ErrMissingField("eventType")
	}
	if errs := validation.IsQualifiedName(et.EventType); len(errs) > 0 {
		return apis.ErrInvalidValue(et.EventType, "eventType")
	}

	if et.Resource == "" {
		return apis.ErrMissingField("resource")
	}

	return nil
}

// Validate validates flow action has a valid formed target or targetURI, but not both.
func (fa *FlowAction) Validate() *apis.FieldError {
	switch {
	case fa.Target != nil && fa.TargetURI != nil:
		return apis.ErrMultipleOneOf("target", "targetURI")
	case fa.Target != nil:
		if errs := validation.IsQualifiedName(fa.Target.Name); len(errs) > 0 {
			return apis.ErrInvalidValue(fa.Target.Name, "name").ViaField("target")
		}
		return nil
	case fa.TargetURI != nil:
		if _, err := url.ParseRequestURI(*fa.TargetURI); err != nil {
			return apis.ErrInvalidValue(*fa.TargetURI, "targetURI")
		}
		return nil
	default:
		return apis.ErrMissingOneOf("target", "targetURI")
	}
}
