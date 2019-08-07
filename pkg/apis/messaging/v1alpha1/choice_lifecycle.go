/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/pkg/apis"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

var pChoiceCondSet = apis.NewLivingConditionSet(ChoiceConditionReady, ChoiceConditionChannelsReady, ChoiceConditionSubscriptionsReady, ChoiceConditionAddressable)

const (
	// ChoiceConditionReady has status True when all subconditions below have been set to True.
	ChoiceConditionReady = apis.ConditionReady

	// ChoiceConditionChannelsReady has status True when all the channels created as part of
	// this choice are ready.
	ChoiceConditionChannelsReady apis.ConditionType = "ChannelsReady"

	// ChoiceConditionSubscriptionsReady has status True when all the subscriptions created as part of
	// this choice are ready.
	ChoiceConditionSubscriptionsReady apis.ConditionType = "SubscriptionsReady"

	// ChoiceConditionAddressable has status true when this Choice meets
	// the Addressable contract and has a non-empty hostname.
	ChoiceConditionAddressable apis.ConditionType = "Addressable"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *ChoiceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pChoiceCondSet.Manage(ps).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ps *ChoiceStatus) IsReady() bool {
	return pChoiceCondSet.Manage(ps).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *ChoiceStatus) InitializeConditions() {
	pChoiceCondSet.Manage(ps).InitializeConditions()
}

// PropagateSubscriptionStatuses sets the ChoiceConditionSubscriptionsReady based on
// the status of the incoming subscriptions.
func (ps *ChoiceStatus) PropagateSubscriptionStatuses(filterSubscriptions []*eventingv1alpha1.Subscription, subscriptions []*eventingv1alpha1.Subscription) {
	if ps.CaseStatuses == nil {
		ps.CaseStatuses = make([]ChoiceCaseStatus, len(subscriptions))
	}
	allReady := true
	// If there are no subscriptions, treat that as a False case. Could go either way, but this seems right.
	if len(subscriptions) == 0 {
		allReady = false
	}

	for i, s := range subscriptions {
		ps.CaseStatuses[i].SubscriptionStatus = ChoiceSubscriptionStatus{
			Subscription: corev1.ObjectReference{
				APIVersion: s.APIVersion,
				Kind:       s.Kind,
				Name:       s.Name,
				Namespace:  s.Namespace,
			},
		}

		readyCondition := s.Status.GetCondition(eventingv1alpha1.SubscriptionConditionReady)
		if readyCondition != nil {
			ps.CaseStatuses[i].SubscriptionStatus.ReadyCondition = *readyCondition
			if readyCondition.Status != corev1.ConditionTrue {
				allReady = false
			}
		} else {
			allReady = false
		}

		fs := filterSubscriptions[i]
		ps.CaseStatuses[i].FilterSubscriptionStatus = ChoiceSubscriptionStatus{
			Subscription: corev1.ObjectReference{
				APIVersion: fs.APIVersion,
				Kind:       fs.Kind,
				Name:       fs.Name,
				Namespace:  fs.Namespace,
			},
		}
		readyCondition = fs.Status.GetCondition(eventingv1alpha1.SubscriptionConditionReady)
		if readyCondition != nil {
			ps.CaseStatuses[i].FilterSubscriptionStatus.ReadyCondition = *readyCondition
			if readyCondition.Status != corev1.ConditionTrue {
				allReady = false
			}
		} else {
			allReady = false
		}

	}
	if allReady {
		pChoiceCondSet.Manage(ps).MarkTrue(ChoiceConditionSubscriptionsReady)
	} else {
		ps.MarkSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none")
	}
}

// PropagateChannelStatuses sets the ChannelStatuses and ChoiceConditionChannelsReady based on the
// status of the incoming channels.
func (ps *ChoiceStatus) PropagateChannelStatuses(ingressChannel *duckv1alpha1.Channelable, channels []*duckv1alpha1.Channelable) {
	if ps.CaseStatuses == nil {
		ps.CaseStatuses = make([]ChoiceCaseStatus, len(channels))
	}
	allReady := true

	ps.IngressChannelStatus.Channel = corev1.ObjectReference{
		APIVersion: ingressChannel.APIVersion,
		Kind:       ingressChannel.Kind,
		Name:       ingressChannel.Name,
		Namespace:  ingressChannel.Namespace,
	}

	address := ingressChannel.Status.AddressStatus.Address
	if address != nil {
		ps.IngressChannelStatus.ReadyCondition = apis.Condition{Type: apis.ConditionReady, Status: corev1.ConditionTrue}
	} else {
		ps.IngressChannelStatus.ReadyCondition = apis.Condition{Type: apis.ConditionReady, Status: corev1.ConditionFalse, Reason: "NotAddressable", Message: "Channel is not addressable"}
		allReady = false
	}
	// Propagate ingress channel address to Choice
	ps.setAddress(address)

	for i, c := range channels {
		ps.CaseStatuses[i].FilterChannelStatus = ChoiceChannelStatus{
			Channel: corev1.ObjectReference{
				APIVersion: c.APIVersion,
				Kind:       c.Kind,
				Name:       c.Name,
				Namespace:  c.Namespace,
			},
		}
		// TODO: Once the addressable has a real status to dig through, use that here instead of
		// addressable, because it might be addressable but not ready.
		address := c.Status.AddressStatus.Address
		if address != nil {
			ps.CaseStatuses[i].FilterChannelStatus.ReadyCondition = apis.Condition{Type: apis.ConditionReady, Status: corev1.ConditionTrue}
		} else {
			ps.CaseStatuses[i].FilterChannelStatus.ReadyCondition = apis.Condition{Type: apis.ConditionReady, Status: corev1.ConditionFalse, Reason: "NotAddressable", Message: "Channel is not addressable"}
			allReady = false
		}
	}
	if allReady {
		pChoiceCondSet.Manage(ps).MarkTrue(ChoiceConditionChannelsReady)
	} else {
		ps.MarkChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none")
	}
}

func (ps *ChoiceStatus) MarkChannelsNotReady(reason, messageFormat string, messageA ...interface{}) {
	pChoiceCondSet.Manage(ps).MarkFalse(ChoiceConditionChannelsReady, reason, messageFormat, messageA...)
}

func (ps *ChoiceStatus) MarkSubscriptionsNotReady(reason, messageFormat string, messageA ...interface{}) {
	pChoiceCondSet.Manage(ps).MarkFalse(ChoiceConditionSubscriptionsReady, reason, messageFormat, messageA...)
}

func (ps *ChoiceStatus) MarkAddressableNotReady(reason, messageFormat string, messageA ...interface{}) {
	pChoiceCondSet.Manage(ps).MarkFalse(ChoiceConditionAddressable, reason, messageFormat, messageA...)
}

func (ps *ChoiceStatus) setAddress(address *pkgduckv1alpha1.Addressable) {
	ps.Address = address

	if address == nil {
		pChoiceCondSet.Manage(ps).MarkFalse(ChoiceConditionAddressable, "emptyHostname", "hostname is the empty string")
		return
	}
	if address.URL != nil || address.Hostname != "" {
		pChoiceCondSet.Manage(ps).MarkTrue(ChoiceConditionAddressable)
	} else {
		ps.Address.Hostname = ""
		ps.Address.URL = nil
		pChoiceCondSet.Manage(ps).MarkFalse(ChoiceConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}
