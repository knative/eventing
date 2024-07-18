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

package testing

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// ParallelOption enables further configuration of a Parallel.
type FlowsParallelOption func(*flowsv1.Parallel)

// NewFlowsParallel creates an Parallel with ParallelOptions.
func NewFlowsParallel(name, namespace string, popt ...FlowsParallelOption) *flowsv1.Parallel {
	p := &flowsv1.Parallel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: flowsv1.ParallelSpec{},
	}
	for _, opt := range popt {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func WithInitFlowsParallelConditions(p *flowsv1.Parallel) {
	p.Status.InitializeConditions()
}

func WithFlowsParallelGeneration(gen int64) FlowsParallelOption {
	return func(s *flowsv1.Parallel) {
		s.Generation = gen
	}
}

func WithFlowsParallelStatusObservedGeneration(gen int64) FlowsParallelOption {
	return func(s *flowsv1.Parallel) {
		s.Status.ObservedGeneration = gen
	}
}

func WithFlowsParallelDeleted(p *flowsv1.Parallel) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithFlowsParallelChannelTemplateSpec(cts *messagingv1.ChannelTemplateSpec) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Spec.ChannelTemplate = cts
	}
}

func WithFlowsParallelBranches(branches []flowsv1.ParallelBranch) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Spec.Branches = branches
	}
}

func WithFlowsParallelReply(reply *duckv1.Destination) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Spec.Reply = reply
	}
}

func WithFlowsParallelBranchStatuses(branchStatuses []flowsv1.ParallelBranchStatus) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.BranchStatuses = branchStatuses
	}
}

func WithFlowsParallelIngressChannelStatus(status flowsv1.ParallelChannelStatus) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.IngressChannelStatus = status
	}
}

func WithFlowsParallelChannelsNotReady(reason, message string) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.MarkChannelsNotReady(reason, message)
	}
}

func WithFlowsParallelSubscriptionsNotReady(reason, message string) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.MarkSubscriptionsNotReady(reason, message)
	}
}

func WithFlowsParallelAddressableNotReady(reason, message string) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.MarkAddressableNotReady(reason, message)
	}
}

func WithFlowsParallelEventPoliciesReady() FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.MarkEventPoliciesTrue()
	}
}

func WithFlowsParallelEventPoliciesNotReady(reason, message string) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.MarkEventPoliciesFailed(reason, message)
	}
}

func WithFlowsParallelEventPoliciesReadyBecauseOIDCDisabled() FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.MarkEventPoliciesTrueWithReason("OIDCDisabled", "Feature %q must be enabled to support Authorization", feature.OIDCAuthentication)
	}
}

func WithFlowsParallelEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled() FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		p.Status.MarkEventPoliciesTrueWithReason("DefaultAuthorizationMode", "Default authz mode is %q", feature.AuthorizationAllowSameNamespace)
	}
}

func WithFlowsParallelEventPoliciesListed(policyNames ...string) FlowsParallelOption {
	return func(p *flowsv1.Parallel) {
		for _, name := range policyNames {
			p.Status.Policies = append(p.Status.Policies, eventingduckv1.AppliedEventPolicyRef{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Name:       name,
			})
		}
	}
}
