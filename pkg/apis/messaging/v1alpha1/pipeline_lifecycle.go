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
	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
)

var pCondSet = apis.NewLivingConditionSet(PipelineConditionReady, PipelineConditionChannelsReady, PipelineConditionSubscriptionsReady)

const (
	// PipelineConditionReady has status True when all subconditions below have been set to True.
	PipelineConditionReady = apis.ConditionReady

	// PipelineChannelsReady has status True when all the channels created as part of
	// this pipeline are ready.
	PipelineConditionChannelsReady apis.ConditionType = "ChannelsReady"

	// PipelineSubscriptionsReady has status True when all the subscriptions created as part of
	// this pipeline are ready.
	PipelineConditionSubscriptionsReady apis.ConditionType = "SubscriptionsReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *PipelineStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pCondSet.Manage(ps).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ps *PipelineStatus) IsReady() bool {
	return pCondSet.Manage(ps).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *PipelineStatus) InitializeConditions() {
	pCondSet.Manage(ps).InitializeConditions()
}

// PropagateSubscriptionStatuses sets the SubscriptionStatuses and PipelineConditionSubscriptionsReady based on
// the status of the incoming subscriptions.
func (ps *PipelineStatus) PropagateSubscriptionStatuses(subscriptions []*eventingv1alpha1.Subscription) {
	ps.SubscriptionStatuses = make([]PipelineSubscriptionStatus, len(subscriptions))
	allReady := true
	for i, s := range subscriptions {
		ps.SubscriptionStatuses[i] = PipelineSubscriptionStatus{
			Subscription: corev1.ObjectReference{
				APIVersion: s.APIVersion,
				Kind:       s.Kind,
				Name:       s.Name,
			},
		}
		readyCondition := s.Status.GetCondition(eventingv1alpha1.SubscriptionConditionReady)
		if readyCondition != nil {
			ps.SubscriptionStatuses[i].ReadyCondition = *readyCondition
			if readyCondition.Status != corev1.ConditionTrue {
				allReady = false
			}
		} else {
			allReady = false
		}

	}
	if allReady {
		pCondSet.Manage(ps).MarkTrue(PipelineConditionSubscriptionsReady)
	}
}

// PropagateChannelStatuses sets the ChannelStatuses and PipelineConditionChannelsReady based on the
// status of the incoming channels.
func (ps *PipelineStatus) PropagateChannelStatuses(channels []*duckv1alpha1.Channelable) {
	ps.ChannelStatuses = make([]PipelineChannelStatus, len(channels))
	allReady := true
	for i, s := range channels {
		ps.ChannelStatuses[i] = PipelineChannelStatus{
			Channel: corev1.ObjectReference{
				APIVersion: s.APIVersion,
				Kind:       s.Kind,
				Name:       s.Name,
			},
		}
		address := s.Status.AddressStatus.Address
		// TODO: Once channealables actually display status, check it here.
		if address == nil {
			allReady = false
		}
	}
	if allReady {
		pCondSet.Manage(ps).MarkTrue(PipelineConditionChannelsReady)
	}
}
