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
	"k8s.io/utils/ptr"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/pkg/kmeta"
)

const (
	BackingChannelEventPolicyLabelPrefix = "eventing.knative.dev/"
	OIDCBrokerSub                        = "system:serviceaccount:knative-eventing:mt-broker-ingress-oidc"
	brokerAPIVersion                     = "eventing.knative.dev/v1"
	version                              = "v1"
	brokerKind                           = "Broker"
	brokerGroup                          = "eventing.knative.dev"
)

func MakeEventPolicyForBackingChannel(b *eventingv1.Broker, backingChannel *eventingduckv1.Channelable) *eventingv1alpha1.EventPolicy {
	return &eventingv1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backingChannel.Namespace,
			Name:      BrokerEventPolicyName(b.Name, backingChannel.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: brokerAPIVersion,
					Kind:       brokerKind,
					Name:       b.Name,
				},
			},
			Labels: LabelsForBackingChannelsEventPolicy(b),
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
			From: []eventingv1alpha1.EventPolicySpecFrom{
				{
					Sub: ptr.To(OIDCBrokerSub),
				},
			},
		},
	}
}

func LabelsForBackingChannelsEventPolicy(broker *eventingv1.Broker) map[string]string {
	return map[string]string{
		BackingChannelEventPolicyLabelPrefix + "broker-group":   brokerGroup,
		BackingChannelEventPolicyLabelPrefix + "broker-version": version,
		BackingChannelEventPolicyLabelPrefix + "broker-kind":    brokerKind,
		BackingChannelEventPolicyLabelPrefix + "broker-name":    broker.Name,
	}
}

func BrokerEventPolicyName(brokerName, channelName string) string {
	return kmeta.ChildName(brokerName, "-ep-"+channelName)
}
