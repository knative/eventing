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

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"
	sinksv1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
)

// JobSinkOption enables further configuration of a JobSink.
type JobSinkOption func(*sinksv1alpha1.JobSink)

// NewJobSink creates a JobSink with JobSinkOptions.
func NewJobSink(name, namespace string, o ...JobSinkOption) *sinksv1alpha1.JobSink {
	js := &sinksv1alpha1.JobSink{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(js)
	}
	js.SetDefaults(context.Background())
	return js
}

// WithInitJobSinkConditions initializes the JobSink's conditions.
func WithInitJobSinkConditions(js *sinksv1alpha1.JobSink) {
	js.Status.InitializeConditions()
}

func WithJobSinkFinalizers(finalizers ...string) JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.Finalizers = finalizers
	}
}

func WithJobSinkResourceVersion(rv string) JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.ResourceVersion = rv
	}
}

func WithJobSinkGeneration(gen int64) JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.Generation = gen
	}
}

// WithJobSinkJob sets the JobSink's Job.
func WithJobSinkJob(job *batchv1.Job) JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.Spec.Job = job
	}
}

// WithJobSinkEventPoliciesReady sets the JobSink's EventPoliciesReady condition to true.
func WithJobSinkEventPoliciesReady() JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.Status.MarkEventPoliciesTrue()
	}
}

// WithJobSinkEventPoliciesNotReady sets the JobSink's EventPoliciesReady condition to false.
func WithJobSinkEventPoliciesNotReady(reason, message string) JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.Status.MarkEventPoliciesFailed(reason, message)
	}
}

// WithJobSinkEventPoliciesListed adds Ready EventPolicies to the JobSink's status.
func WithJobSinkEventPoliciesListed(policyNames ...string) JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		for _, policyName := range policyNames {
			js.Status.Policies = append(js.Status.Policies, eventingduckv1.AppliedEventPolicyRef{
				Name:       policyName,
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
			})
		}
	}
}

// WithJobSinkEventPoliciesReadyBecauseNoPolicy() sets the JobSink's EventPoliciesReady condition to true with reason.
func WithJobSinkEventPoliciesReadyBecauseOIDCDisabled() JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.Status.DeepCopy().MarkEventPoliciesTrueWithReason("OIDCDisabled", "Feature %q must be enabled to support Authorization", feature.OIDCAuthentication)
	}
}

// WithJobSinkEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled() sets the JobSink's EventPoliciesReady condition to true with reason.
func WithJobSinkEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled() JobSinkOption {
	return func(js *sinksv1alpha1.JobSink) {
		js.Status.MarkEventPoliciesTrueWithReason("DefaultAuthorizationMode", "Default authz mode is %q", feature.AuthorizationAllowSameNamespace)
	}
}
