/*
Copyright 2018 The Knative Authors

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

package base

import (
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

// IsStatusReady will check if the ReadyCondition is set to True.
// TODO(Fredy-Z): we probably don't want this.
func IsStatusReady(status duckv1alpha1.Status) (bool, error) {
	condSet := duckv1alpha1.NewLivingConditionSet()
	return condSet.Manage(status).IsHappy(), nil
}

// IsChannelReady will check the status conditions of the Channel and return true
// if the Channel is ready.
func IsChannelReady(c *eventingv1alpha1.Channel) (bool, error) {
	return c.Status.IsReady(), nil
}

// ChannelsReady will check the status conditions of the channel list and return true
// if all channels are Ready.
func ChannelsReady(channelList *eventingv1alpha1.ChannelList) (bool, error) {
	for _, channel := range channelList.Items {
		if isReady, _ := IsChannelReady(&channel); !isReady {
			return false, nil
		}
	}
	return true, nil
}

// IsSubscriptionReady will check the status conditions of the Subscription and
// return true if the Subscription is ready.
func IsSubscriptionReady(s *eventingv1alpha1.Subscription) (bool, error) {
	return s.Status.IsReady(), nil
}

// SubscriptionsReady will check the status conditions of the subscription list and return true
// if all subscriptions are Ready.
func SubscriptionsReady(subscriptionList *eventingv1alpha1.SubscriptionList) (bool, error) {
	for _, subscription := range subscriptionList.Items {
		if isReady, _ := IsSubscriptionReady(&subscription); !isReady {
			return false, nil
		}
	}
	return true, nil
}

// IsBrokerReady will check the status conditions of the Broker and return true
// if the Broker is ready.
func IsBrokerReady(b *eventingv1alpha1.Broker) (bool, error) {
	return b.Status.IsReady(), nil
}

// BrokersReady will check the status conditions of the broker list and return true
// if all brokers are Ready.
func BrokersReady(brokerList *eventingv1alpha1.BrokerList) (bool, error) {
	for _, broker := range brokerList.Items {
		if isReady, _ := IsBrokerReady(&broker); !isReady {
			return false, nil
		}
	}
	return true, nil
}

// IsTriggerReady will check the status conditions of the Trigger and
// return true if the Trigger is ready.
func IsTriggerReady(t *eventingv1alpha1.Trigger) (bool, error) {
	return t.Status.IsReady(), nil
}

// TriggersReady will check the status conditions of the trigger list and return true
// if all triggers are Ready.
func TriggersReady(triggerList *eventingv1alpha1.TriggerList) (bool, error) {
	for _, trigger := range triggerList.Items {
		if isReady, _ := IsTriggerReady(&trigger); !isReady {
			return false, nil
		}
	}
	return true, nil
}
