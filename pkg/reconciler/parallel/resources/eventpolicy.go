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

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/kmeta"
)

const (
	ParallelChannelEventPolicyLabelPrefix = "flows.knative.dev/"
	parallelKind                          = "Parallel"
)

func MakeEventPolicyForParallelChannel(p *flowsv1.Parallel, channel *eventingduckv1.Channelable, subscription *messagingv1.Subscription) *eventingv1alpha1.EventPolicy {
	return &eventingv1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: channel.Namespace,
			Name:      ParallelEventPolicyName(p.Name, channel.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: flowsv1.SchemeGroupVersion.String(),
					Kind:       parallelKind,
					Name:       p.Name,
				},
			},
			Labels: LabelsForParallelChannelsEventPolicy(p.Name),
		},
		Spec: eventingv1alpha1.EventPolicySpec{
			To: []eventingv1alpha1.EventPolicySpecTo{
				{
					Ref: &eventingv1alpha1.EventPolicyToReference{
						APIVersion: channel.APIVersion,
						Kind:       channel.Kind,
						Name:       channel.Name,
					},
				},
			},
			From: []eventingv1alpha1.EventPolicySpecFrom{
				{
					Ref: &eventingv1alpha1.EventPolicyFromReference{
						APIVersion: subscription.APIVersion,
						Kind:       subscription.Kind,
						Name:       subscription.Name,
						Namespace:  subscription.Namespace,
					},
				},
			},
		},
	}
}

func LabelsForParallelChannelsEventPolicy(parallelName string) map[string]string {
	return map[string]string{
		ParallelChannelEventPolicyLabelPrefix + "parallel-group":   flowsv1.SchemeGroupVersion.Group,
		ParallelChannelEventPolicyLabelPrefix + "parallel-version": flowsv1.SchemeGroupVersion.Version,
		ParallelChannelEventPolicyLabelPrefix + "parallel-kind":    parallelKind,
		ParallelChannelEventPolicyLabelPrefix + "parallel-name":    parallelName,
	}
}

func ParallelEventPolicyName(parallelName, channelName string) string {
	// if channel name is empty, it means the event policy is for the output channel
	if channelName == "" {
		return kmeta.ChildName(parallelName, "-ep") // no need to add the channel name
	} else {
		return kmeta.ChildName(parallelName, "-ep-"+channelName)
	}
}

// MakeEventPolicyForParallelIngressChannel creates an EventPolicy for the ingress channel of a Parallel.
func MakeEventPolicyForParallelIngressChannel(p *flowsv1.Parallel, ingressChannel *eventingduckv1.Channelable, parallelPolicy *eventingv1alpha1.EventPolicy) *eventingv1alpha1.EventPolicy {
	return &eventingv1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ingressChannel.Namespace,
			Name:      ParallelEventPolicyName(p.Name, ingressChannel.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: flowsv1.SchemeGroupVersion.String(),
					Kind:       parallelKind,
					Name:       p.Name,
				},
			},
			Labels: LabelsForParallelChannelsEventPolicy(p.Name),
		},
		Spec: eventingv1alpha1.EventPolicySpec{
			To: []eventingv1alpha1.EventPolicySpecTo{
				{
					Ref: &eventingv1alpha1.EventPolicyToReference{
						APIVersion: ingressChannel.APIVersion,
						Kind:       ingressChannel.Kind,
						Name:       ingressChannel.Name,
					},
				},
			},
			From: parallelPolicy.Spec.From,
		},
	}
}
