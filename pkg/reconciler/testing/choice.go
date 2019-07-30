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

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChoiceOption enables further configuration of a Choice.
type ChoiceOption func(*v1alpha1.Choice)

// NewChoice creates an Choice with ChoiceOptions.
func NewChoice(name, namespace string, popt ...ChoiceOption) *v1alpha1.Choice {
	p := &v1alpha1.Choice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ChoiceSpec{},
	}
	for _, opt := range popt {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func WithInitChoiceConditions(p *v1alpha1.Choice) {
	p.Status.InitializeConditions()
}

func WithChoiceDeleted(p *v1alpha1.Choice) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithChoiceChannelTemplateSpec(cts *eventingduck.ChannelTemplateSpec) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Spec.ChannelTemplate = cts
	}
}

func WithChoiceCases(cases []v1alpha1.ChoiceCase) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Spec.Cases = cases
	}
}

func WithChoiceReply(reply *corev1.ObjectReference) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Spec.Reply = reply
	}
}

func WithChoiceCaseStatuses(caseStatuses []v1alpha1.ChoiceCaseStatus) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Status.CaseStatuses = caseStatuses
	}
}

func WithChoiceIngressChannelStatus(status v1alpha1.ChoiceChannelStatus) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Status.IngressChannelStatus = status
	}
}

func WithChoiceChannelsNotReady(reason, message string) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Status.MarkChannelsNotReady(reason, message)
	}
}

func WithChoiceSubscriptionsNotReady(reason, message string) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Status.MarkSubscriptionsNotReady(reason, message)
	}
}

func WithChoiceAddressableNotReady(reason, message string) ChoiceOption {
	return func(p *v1alpha1.Choice) {
		p.Status.MarkAddressableNotReady(reason, message)
	}
}
