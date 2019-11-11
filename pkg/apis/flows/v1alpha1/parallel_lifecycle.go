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
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/pkg/apis"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

var pParallelCondSet = apis.NewLivingConditionSet(ParallelConditionReady, ParallelConditionChannelsReady, ParallelConditionSubscriptionsReady, ParallelConditionAddressable)

const (
	// StatusConditionTypeDeprecated is the status.conditions.type used to provide deprecation
	// warnings.
	StatusConditionTypeDeprecated = "Deprecated"
)

const (
	// ParallelConditionReady has status True when all subconditions below have been set to True.
	ParallelConditionReady = apis.ConditionReady

	// ParallelConditionChannelsReady has status True when all the channels created as part of
	// this parallel are ready.
	ParallelConditionChannelsReady apis.ConditionType = "ChannelsReady"

	// ParallelConditionSubscriptionsReady has status True when all the subscriptions created as part of
	// this parallel are ready.
	ParallelConditionSubscriptionsReady apis.ConditionType = "SubscriptionsReady"

	// ParallelConditionAddressable has status true when this Parallel meets
	// the Addressable contract and has a non-empty hostname.
	ParallelConditionAddressable apis.ConditionType = "Addressable"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *ParallelStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pParallelCondSet.Manage(ps).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ps *ParallelStatus) IsReady() bool {
	return pParallelCondSet.Manage(ps).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *ParallelStatus) InitializeConditions() {
	pParallelCondSet.Manage(ps).InitializeConditions()
}

// PropagateSubscriptionStatuses sets the ParallelConditionSubscriptionsReady based on
// the status of the incoming subscriptions.
func (ps *ParallelStatus) PropagateSubscriptionStatuses(filterSubscriptions []*messagingv1alpha1.Subscription, subscriptions []*messagingv1alpha1.Subscription) {
	if ps.BranchStatuses == nil {
		ps.BranchStatuses = make([]ParallelBranchStatus, len(subscriptions))
	}
	allReady := true
	// If there are no subscriptions, treat that as a False branch. Could go either way, but this seems right.
	if len(subscriptions) == 0 {
		allReady = false
	}

	for i, s := range subscriptions {
		ps.BranchStatuses[i].SubscriptionStatus = ParallelSubscriptionStatus{
			Subscription: corev1.ObjectReference{
				APIVersion: s.APIVersion,
				Kind:       s.Kind,
				Name:       s.Name,
				Namespace:  s.Namespace,
			},
		}

		readyCondition := s.Status.GetCondition(messagingv1alpha1.SubscriptionConditionReady)
		if readyCondition != nil {
			ps.BranchStatuses[i].SubscriptionStatus.ReadyCondition = *readyCondition
			if readyCondition.Status != corev1.ConditionTrue {
				allReady = false
			}
		} else {
			allReady = false
		}

		fs := filterSubscriptions[i]
		ps.BranchStatuses[i].FilterSubscriptionStatus = ParallelSubscriptionStatus{
			Subscription: corev1.ObjectReference{
				APIVersion: fs.APIVersion,
				Kind:       fs.Kind,
				Name:       fs.Name,
				Namespace:  fs.Namespace,
			},
		}
		readyCondition = fs.Status.GetCondition(messagingv1alpha1.SubscriptionConditionReady)
		if readyCondition != nil {
			ps.BranchStatuses[i].FilterSubscriptionStatus.ReadyCondition = *readyCondition
			if readyCondition.Status != corev1.ConditionTrue {
				allReady = false
			}
		} else {
			allReady = false
		}

	}
	if allReady {
		pParallelCondSet.Manage(ps).MarkTrue(ParallelConditionSubscriptionsReady)
	} else {
		ps.MarkSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none")
	}
}

// PropagateChannelStatuses sets the ChannelStatuses and ParallelConditionChannelsReady based on the
// status of the incoming channels.
func (ps *ParallelStatus) PropagateChannelStatuses(ingressChannel *duckv1alpha1.Channelable, channels []*duckv1alpha1.Channelable) {
	if ps.BranchStatuses == nil {
		ps.BranchStatuses = make([]ParallelBranchStatus, len(channels))
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
	ps.setAddress(address)

	for i, c := range channels {
		ps.BranchStatuses[i].FilterChannelStatus = ParallelChannelStatus{
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
			ps.BranchStatuses[i].FilterChannelStatus.ReadyCondition = apis.Condition{Type: apis.ConditionReady, Status: corev1.ConditionTrue}
		} else {
			ps.BranchStatuses[i].FilterChannelStatus.ReadyCondition = apis.Condition{Type: apis.ConditionReady, Status: corev1.ConditionFalse, Reason: "NotAddressable", Message: "Channel is not addressable"}
			allReady = false
		}
	}
	if allReady {
		pParallelCondSet.Manage(ps).MarkTrue(ParallelConditionChannelsReady)
	} else {
		ps.MarkChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none")
	}
}

func (ps *ParallelStatus) MarkChannelsNotReady(reason, messageFormat string, messageA ...interface{}) {
	pParallelCondSet.Manage(ps).MarkFalse(ParallelConditionChannelsReady, reason, messageFormat, messageA...)
}

func (ps *ParallelStatus) MarkSubscriptionsNotReady(reason, messageFormat string, messageA ...interface{}) {
	pParallelCondSet.Manage(ps).MarkFalse(ParallelConditionSubscriptionsReady, reason, messageFormat, messageA...)
}

func (ps *ParallelStatus) MarkAddressableNotReady(reason, messageFormat string, messageA ...interface{}) {
	pParallelCondSet.Manage(ps).MarkFalse(ParallelConditionAddressable, reason, messageFormat, messageA...)
}

func (ps *ParallelStatus) setAddress(address *pkgduckv1alpha1.Addressable) {
	if address == nil {
		pParallelCondSet.Manage(ps).MarkFalse(ParallelConditionAddressable, "emptyAddress", "addressable is nil")
		ps.Address = nil
		return
	}
	if address.URL != nil || address.Hostname != "" {
		// But, first, convert it to V1, since Channel Status
		// is using v1alpha1 Address. Note that ConvertUp does
		// not do anything with the passed in Context, so
		// just make one up here.
		v1Address := pkgduckv1.Addressable{}
		if err := address.ConvertUp(context.TODO(), &v1Address); err != nil {
			pCondSet.Manage(ps).MarkFalse(SequenceConditionAddressable, "emptyAddress", "unable to convert channel address up to v1")
			return
		}
		ps.Address = &v1Address
		pParallelCondSet.Manage(ps).MarkTrue(ParallelConditionAddressable)
	} else {
		ps.Address = nil
		pCondSet.Manage(ps).MarkFalse(SequenceConditionAddressable, "emptyHostname", "channel addressable is nil and/or no hostname")
	}
}

// MarkDeprecated adds a warning condition that this object's spec is using deprecated fields
// and will stop working in the future. Note that this does not affect the Ready condition.
func (ps *ParallelStatus) MarkDestinationDeprecatedRef(reason, msg string) {
	dc := apis.Condition{
		Type:               StatusConditionTypeDeprecated,
		Reason:             reason,
		Status:             corev1.ConditionTrue,
		Severity:           apis.ConditionSeverityWarning,
		Message:            msg,
		LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now())},
	}
	for i, c := range ps.Conditions {
		if c.Type == dc.Type {
			ps.Conditions[i] = dc
			return
		}
	}
	ps.Conditions = append(ps.Conditions, dc)
}

// ClearDeprecated removes the StatusConditionTypeDeprecated warning condition. Note that this does not
// affect the Ready condition.
func (ps *ParallelStatus) ClearDeprecated() {
	conds := make([]apis.Condition, 0, len(ps.Conditions))
	for _, c := range ps.Conditions {
		if c.Type != StatusConditionTypeDeprecated {
			conds = append(conds, c)
		}
	}
	ps.Conditions = conds
}
