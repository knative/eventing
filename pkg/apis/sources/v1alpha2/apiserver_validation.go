/*
Copyright 2020 The Knative Authors

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

package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"
)

const (
	// ReferenceMode produces payloads of ObjectReference
	ReferenceMode = "Reference"
	// ResourceMode produces payloads of ResourceEvent
	ResourceMode = "Resource"
)

func (c *ApiServerSource) Validate(ctx context.Context) *apis.FieldError {
	return c.Spec.Validate(ctx).ViaField("spec")
}

func (cs *ApiServerSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if len(cs.Resources) == 0 {
		errs = errs.Also(apis.ErrMissingField("resources"))
	}

	// Validate mode, if can be empty or set as certain value
	switch cs.EventMode {
	case ReferenceMode, ResourceMode:
	// EventMode is valid.
	default:
		errs = errs.Also(apis.ErrInvalidValue(cs.EventMode, "mode"))
	}

	// Validate sink
	errs = errs.Also(cs.Sink.Validate(ctx).ViaField("sink"))

	return errs
}
