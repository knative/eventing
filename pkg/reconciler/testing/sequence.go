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

	eventingduckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SequenceOption enables further configuration of a Sequence.
type SequenceOption func(*v1alpha1.Sequence)

// NewSequence creates an Sequence with SequenceOptions.
func NewSequence(name, namespace string, popt ...SequenceOption) *v1alpha1.Sequence {
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

func WithInitSequenceConditions(p *v1alpha1.Sequence) {
	p.Status.InitializeConditions()
}

func WithSequenceDeleted(p *v1alpha1.Sequence) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithSequenceChannelTemplateSpec(cts eventingduckv1alpha1.ChannelTemplateSpec) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Spec.ChannelTemplate = cts
	}
}

func WithSequenceSteps(steps []eventingv1alpha1.SubscriberSpec) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Spec.Steps = steps
	}
}

func WithSequenceReply(reply *corev1.ObjectReference) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Spec.Reply = reply
	}
}

func WithSequenceSubscriptionStatuses(subscriptionStatuses []v1alpha1.SequenceSubscriptionStatus) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.SubscriptionStatuses = subscriptionStatuses
	}
}

func WithSequenceChannelStatuses(channelStatuses []v1alpha1.SequenceChannelStatus) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.ChannelStatuses = channelStatuses
	}
}

func WithSequenceChannelsNotReady(reason, message string) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.MarkChannelsNotReady(reason, message)
	}
}

func WithSequenceSubscriptionsNotReady(reason, message string) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.MarkSubscriptionsNotReady(reason, message)
	}
}

func WithSequenceAddressableNotReady(reason, message string) SequenceOption {
	return func(p *v1alpha1.Sequence) {
		p.Status.MarkAddressableNotReady(reason, message)
	}
}
