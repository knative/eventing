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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/pkg/kmeta"
)

const (
	BackingChannelEventPolicyLabelPrefix = "messaging.knative.dev/"
)

func MakeEventPolicyForBackingChannel(backingChannel *eventingduckv1.Channelable, parentPolicy *eventingv1alpha1.EventPolicy) *eventingv1alpha1.EventPolicy {
	parentPolicy = parentPolicy.DeepCopy()

	return &eventingv1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backingChannel.Namespace,
			Name:      kmeta.ChildName(fmt.Sprintf("%s-", parentPolicy.Name), backingChannel.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backingChannel.APIVersion,
					Kind:       backingChannel.Kind,
					Name:       backingChannel.Name,
					UID:        backingChannel.UID,
				}, {
					APIVersion: parentPolicy.APIVersion,
					Kind:       parentPolicy.Kind,
					Name:       parentPolicy.Name,
					UID:        parentPolicy.UID,
				},
			},
			Labels: LabelsForBackingChannelsEventPolicy(backingChannel),
		},
		Spec: eventingv1alpha1.EventPolicySpec{
			To: []eventingv1alpha1.EventPolicySpecTo{
				{
					Ref: &eventingv1alpha1.EventPolicyToReference{
						APIVersion: backingChannel.APIVersion,
						Kind:       backingChannel.Kind,
						Name:       backingChannel.Name,
					},
				},
			},
			From: parentPolicy.Spec.From,
		},
	}
}

func LabelsForBackingChannelsEventPolicy(backingChannel *eventingduckv1.Channelable) map[string]string {
	return map[string]string{
		BackingChannelEventPolicyLabelPrefix + "channel-group":   backingChannel.GroupVersionKind().Group,
		BackingChannelEventPolicyLabelPrefix + "channel-version": backingChannel.GroupVersionKind().Version,
		BackingChannelEventPolicyLabelPrefix + "channel-kind":    backingChannel.Kind,
		BackingChannelEventPolicyLabelPrefix + "channel-name":    backingChannel.Name,
	}
}
