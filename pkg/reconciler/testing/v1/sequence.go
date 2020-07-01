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
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// SequenceOption enables further configuration of a Sequence.
type SequenceOption func(*flowsv1.Sequence)

// NewSequence creates an Sequence with SequenceOptions.
func NewSequence(name, namespace string, popt ...SequenceOption) *flowsv1.Sequence {
	p := &flowsv1.Sequence{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: flowsv1.SequenceSpec{},
	}
	for _, opt := range popt {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func WithSequenceGeneration(gen int64) SequenceOption {
	return func(s *flowsv1.Sequence) {
		s.Generation = gen
	}
}

func WithSequenceStatusObservedGeneration(gen int64) SequenceOption {
	return func(s *flowsv1.Sequence) {
		s.Status.ObservedGeneration = gen
	}
}

func WithInitSequenceConditions(p *flowsv1.Sequence) {
	p.Status.InitializeConditions()
}

func WithSequenceDeleted(p *flowsv1.Sequence) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithSequenceChannelTemplateSpec(cts *messagingv1.ChannelTemplateSpec) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Spec.ChannelTemplate = cts
	}
}

func WithSequenceSteps(steps []flowsv1.SequenceStep) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Spec.Steps = steps
	}
}

func WithSequenceReply(reply *duckv1.Destination) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Spec.Reply = reply
	}
}

func WithSequenceSubscriptionStatuses(subscriptionStatuses []flowsv1.SequenceSubscriptionStatus) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Status.SubscriptionStatuses = subscriptionStatuses
	}
}

func WithSequenceChannelStatuses(channelStatuses []flowsv1.SequenceChannelStatus) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Status.ChannelStatuses = channelStatuses
	}
}

func WithSequenceChannelsNotReady(reason, message string) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Status.MarkChannelsNotReady(reason, message)
	}
}

func WithSequenceSubscriptionsNotReady(reason, message string) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Status.MarkSubscriptionsNotReady(reason, message)
	}
}

func WithSequenceAddressableNotReady(reason, message string) SequenceOption {
	return func(p *flowsv1.Sequence) {
		p.Status.MarkAddressableNotReady(reason, message)
	}
}
