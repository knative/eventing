/*
Copyright 2024 The Knative Authors

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/sinks"
)

func (sink *JobSink) Validate(ctx context.Context) *apis.FieldError {
	ctx = apis.WithinParent(ctx, sink.ObjectMeta)
	return sink.Spec.Validate(ctx).ViaField("spec")
}

func (sink *JobSinkSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if sink.Job == nil {
		return errs.Also(apis.ErrMissingOneOf("job"))
	}

	if sink.Job != nil {
		job := sink.Job.DeepCopy()
		job.Name = names.SimpleNameGenerator.GenerateName(apis.ParentMeta(ctx).Name)
		_, err := sinks.GetConfig(ctx).KubeClient.
			BatchV1().
			Jobs(apis.ParentMeta(ctx).Namespace).
			Create(ctx, job, metav1.CreateOptions{
				DryRun:          []string{metav1.DryRunAll},
				FieldValidation: metav1.FieldValidationStrict,
			})
		if err != nil {
			return apis.ErrGeneric(err.Error(), "job")
		}
	}

	return errs
}
