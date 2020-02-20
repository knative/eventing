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
	"knative.dev/eventing/pkg/apis/flows/v1alpha1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// SequenceOption enables further configuration of a Sequence.
type FlowsSequenceOption func(*v1alpha1.Sequence)

// NewSequence creates an Sequence with SequenceOptions.
func NewFlowsSequence(name, namespace string, popt ...FlowsSequenceOption) *v1alpha1.Sequence {
	p := &v1alpha1.Sequence{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.SequenceSpec{},
	}
	for _, opt := range popt {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func WithFlowsSequenceGeneration(gen int64) FlowsSequenceOption {
	return func(s *v1alpha1.Sequence) {
		s.Generation = gen
	}
}

func WithFlowsSequenceStatusObservedGeneration(gen int64) FlowsSequenceOption {
	return func(s *v1alpha1.Sequence) {
		s.Status.ObservedGeneration = gen
	}
}

func WithInitFlowsSequenceConditions(p *v1alpha1.Sequence) {
	p.Status.InitializeConditions()
}

func WithFlowsSequenceDeleted(p *v1alpha1.Sequence) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithFlowsSequenceChannelTemplateSpec(cts *messagingv1beta1.ChannelTemplateSpec) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Spec.ChannelTemplate = cts
	}
}

func WithFlowsSequenceSteps(steps []v1alpha1.SequenceStep) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Spec.Steps = steps
	}
}

func WithFlowsSequenceReply(reply *duckv1.Destination) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Spec.Reply = reply
	}
}

func WithFlowsSequenceSubscriptionStatuses(subscriptionStatuses []v1alpha1.SequenceSubscriptionStatus) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.SubscriptionStatuses = subscriptionStatuses
	}
}

func WithFlowsSequenceChannelStatuses(channelStatuses []v1alpha1.SequenceChannelStatus) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.ChannelStatuses = channelStatuses
	}
}

func WithFlowsSequenceChannelsNotReady(reason, message string) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.MarkChannelsNotReady(reason, message)
	}
}

func WithFlowsSequenceSubscriptionsNotReady(reason, message string) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.MarkSubscriptionsNotReady(reason, message)
	}
}

func WithFlowsSequenceAddressableNotReady(reason, message string) FlowsSequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.MarkAddressableNotReady(reason, message)
	}
}
