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

package resources

import (
	"fmt"

	"github.com/knative/pkg/kmeta"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	v1alpha1 "github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PipelineSubscriptionName(pipelineName string, step int) string {
	return fmt.Sprintf("%s-kn-pipeline-%d", pipelineName, step)
}

func NewSubscription(stepNumber int, p *v1alpha1.Pipeline) *eventingv1alpha1.Subscription {
	r := &eventingv1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      PipelineSubscriptionName(p.Name, stepNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
		},
		Spec: eventingv1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       PipelineChannelName(p.Name, stepNumber),
			},
			Subscriber: &p.Spec.Steps[stepNumber],
		},
	}
	// If it's not the last step, use the next channel as the reply to, if it's the very
	// last one, we'll use the (optional) reply from the Pipeline Spec.
	if stepNumber < len(p.Spec.Steps)-1 {
		r.Spec.Reply = &eventingv1alpha1.ReplyStrategy{
			Channel: &corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       PipelineChannelName(p.Name, stepNumber+1),
			},
		}
	} else if p.Spec.Reply != nil {
		r.Spec.Reply = &eventingv1alpha1.ReplyStrategy{Channel: p.Spec.Reply}
	}
	return r
}
