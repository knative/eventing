/*
Copyright 2019 The Knative Authors

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
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// ParallelOption enables further configuration of a Parallel.
type ParallelOption func(*v1alpha1.Parallel)

// NewParallel creates an Parallel with ParallelOptions.
func NewParallel(name, namespace string, popt ...ParallelOption) *v1alpha1.Parallel {
	p := &v1alpha1.Parallel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ParallelSpec{},
	}
	for _, opt := range popt {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func WithInitParallelConditions(p *v1alpha1.Parallel) {
	p.Status.InitializeConditions()
}

func WithParallelDeleted(p *v1alpha1.Parallel) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithParallelChannelTemplateSpec(cts *eventingduck.ChannelTemplateSpec) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Spec.ChannelTemplate = cts
	}
}

func WithParallelBranches(branches []v1alpha1.ParallelBranch) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Spec.Branches = branches
	}
}

func WithParallelReply(reply *duckv1beta1.Destination) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Spec.Reply = reply
	}
}

func WithParallelBranchStatuses(branchStatuses []v1alpha1.ParallelBranchStatus) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Status.BranchStatuses = branchStatuses
	}
}

func WithParallelDeprecatedReplyStatus() ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Status.MarkDestinationDeprecatedRef("replyDeprecatedRef", "spec.reply.{apiVersion,kind,name} are deprecated and will be removed in 0.11. Use spec.reply.ref instead.")
	}
}

func WithParallelDeprecatedBranchReplyStatus() ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Status.MarkDestinationDeprecatedRef("branchReplyDeprecatedRef", "spec.branches[*].reply.{apiVersion,kind,name} are deprecated and will be removed in 0.11. Use spec.branches[*].reply.ref instead.")
	}
}

func WithParallelIngressChannelStatus(status v1alpha1.ParallelChannelStatus) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Status.IngressChannelStatus = status
	}
}

func WithParallelChannelsNotReady(reason, message string) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Status.MarkChannelsNotReady(reason, message)
	}
}

func WithParallelSubscriptionsNotReady(reason, message string) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Status.MarkSubscriptionsNotReady(reason, message)
	}
}

func WithParallelAddressableNotReady(reason, message string) ParallelOption {
	return func(p *v1alpha1.Parallel) {
		p.Status.MarkAddressableNotReady(reason, message)
	}
}
