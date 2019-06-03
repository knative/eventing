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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineOption enables further configuration of a Pipeline.
type PipelineOption func(*v1alpha1.Pipeline)

// NewPipeline creates an Pipeline with PipelineOptions.
func NewPipeline(name, namespace string, popt ...PipelineOption) *v1alpha1.Pipeline {
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.PipelineSpec{},
	}
	for _, opt := range popt {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func WithInitPipelineConditions(p *v1alpha1.Pipeline) {
	p.Status.InitializeConditions()
}

func WithPipelineDeleted(p *v1alpha1.Pipeline) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithPipelineChannelTemplateSpec(cts v1alpha1.ChannelTemplateSpec) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Spec.ChannelTemplate = cts
	}
}

func WithPipelineSteps(steps []eventingv1alpha1.SubscriberSpec) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Spec.Steps = steps
	}
}

func WithPipelineSubscriptionStatuses(subscriptionStatuses []v1alpha1.PipelineSubscriptionStatus) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.SubscriptionStatuses = subscriptionStatuses
	}
}

func WithPipelineChannelStatuses(channelStatuses []v1alpha1.PipelineChannelStatus) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.ChannelStatuses = channelStatuses
	}
}

func WithPipelineChannelsNotReady(reason, message string) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkChannelsNotReady(reason, message)
	}
}

func WithPipelineSubscriptionssNotReady(reason, message string) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkSubscriptionsNotReady(reason, message)
	}
}
