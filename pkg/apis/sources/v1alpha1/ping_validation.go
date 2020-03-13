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

package v1alpha1

import (
	"context"

	"github.com/robfig/cron"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

func (c *PingSource) Validate(ctx context.Context) *apis.FieldError {
	errs := c.Spec.Validate(ctx).ViaField("spec")
	return v1alpha2.ValidateAnnotations(errs, c.Annotations)
}

func (cs *PingSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if _, err := cron.ParseStandard(cs.Schedule); err != nil {
		fe := apis.ErrInvalidValue(cs.Schedule, "schedule")
		errs = errs.Also(fe)
	}

	if cs.Sink == nil {
		fe := apis.ErrMissingField("sink")
		errs = errs.Also(fe)
	} else if fe := cs.Sink.Validate(ctx); fe != nil {
		errs = errs.Also(fe.ViaField("sink"))
	}
	return errs
}
