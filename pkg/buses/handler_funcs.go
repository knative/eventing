/*
 * Copyright 2018 The Knative Authors
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

package buses

import (
	"fmt"

	"go.uber.org/zap"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/controller/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a resource is synced
	successSynced = "Synced"
	// ErrResourceSync is used as part of the Event 'reason' when a resource fails
	// to sync.
	errResourceSync = "ErrResourceSync"
)

// ResolvedParameters is a map containing parameter names and the resolved
// values from attributes or parameter defaults.
type ResolvedParameters = map[string]string

// EventHandlerFuncs is a set of handler functions that are called when a bus
// requires sync, channels are provisioned/unprovisioned, or a subscription is
// created or deleted, or if one of the relevant resources is changed.
type EventHandlerFuncs struct {
	// BusFunc is invoked when the Bus requires sync.
	BusFunc func(bus BusReference) error

	// ProvisionFunc is invoked when a new Channel should be provisioned or when
	// the attributes change.
	ProvisionFunc func(channel ChannelReference, parameters ResolvedParameters) error

	// UnprovisionFunc in invoked when a Channel should be deleted.
	UnprovisionFunc func(channel ChannelReference) error

	// SubscribeFunc is invoked when a new Subscription should be set up or when
	// the attributes change.
	SubscribeFunc func(channel ChannelReference, subscription SubscriptionReference, parameters ResolvedParameters) error

	// UnsubscribeFunc is invoked when a Subscription should be deleted.
	UnsubscribeFunc func(channel ChannelReference, subscription SubscriptionReference) error

	// ReceiveMessageFunc is invoked when a Message is received on a Channel
	ReceiveMessageFunc func(channel ChannelReference, message *Message) error

	logger *zap.SugaredLogger
}

func (h EventHandlerFuncs) onBus(bus channelsv1alpha1.GenericBus, reconciler *Reconciler) error {
	if h.BusFunc == nil {
		return nil
	}
	ref := NewBusReference(bus)
	err := h.BusFunc(ref)
	if err != nil {
		reconciler.RecordBusEventf(corev1.EventTypeWarning, errResourceSync, "Error syncing Bus: %s", err)
	} else {
		reconciler.RecordBusEventf(corev1.EventTypeNormal, successSynced, "Bus synched successfully")
	}
	return err
}

func (h EventHandlerFuncs) onProvision(channel *channelsv1alpha1.Channel, reconciler *Reconciler) error {
	if h.ProvisionFunc == nil {
		return nil
	}
	parameters, err := h.resolveChannelParameters(reconciler.bus.GetSpec(), channel.Spec)
	if err != nil {
		return err
	}
	channelCopy := channel.DeepCopy()
	var cond *channelsv1alpha1.ChannelCondition
	ref := NewChannelReference(channel)
	err = h.ProvisionFunc(ref, parameters)
	if err != nil {
		reconciler.RecordChannelEventf(ref, corev1.EventTypeWarning, errResourceSync, "Error provisioning channel: %s", err)
		cond = util.NewChannelCondition(channelsv1alpha1.ChannelProvisioned, corev1.ConditionFalse, errResourceSync, err.Error())
	} else {
		reconciler.RecordChannelEventf(ref, corev1.EventTypeNormal, successSynced, "Channel provisioned successfully")
		cond = util.NewChannelCondition(channelsv1alpha1.ChannelProvisioned, corev1.ConditionTrue, successSynced, "Channel provisioned successfully")
	}
	util.SetChannelCondition(&channelCopy.Status, *cond)
	util.ConsolidateChannelCondition(&channelCopy.Status)
	if !equality.Semantic.DeepEqual(channel.Status, channelCopy.Status) {
		// status has changed, update channel
		_, errS := reconciler.eventingClient.ChannelsV1alpha1().Channels(channel.Namespace).Update(channelCopy)
		if errS != nil {
			h.logger.Warnf("Could not update channel status: %v", errS)
			if err != nil {
				return fmt.Errorf("error provisioning channel (%v); error updating channel status (%v)", err, errS)
			}
			return errS
		}
	}
	return err
}

func (h EventHandlerFuncs) onUnprovision(channel *channelsv1alpha1.Channel, reconciler *Reconciler) error {
	if h.UnprovisionFunc == nil {
		return nil
	}
	ref := NewChannelReference(channel)
	if err := h.UnprovisionFunc(ref); err != nil {
		reconciler.RecordChannelEventf(ref, corev1.EventTypeWarning, errResourceSync, "Error unprovisioning channel: %s", err)
		return err
	}
	reconciler.RecordChannelEventf(ref, corev1.EventTypeNormal, successSynced, "Channel unprovisioned successfully")
	// skip updating status conditions since the channel was deleted
	return nil
}

func (h EventHandlerFuncs) onSubscribe(subscription *channelsv1alpha1.Subscription, reconciler *Reconciler) error {
	if h.SubscribeFunc == nil {
		return nil
	}
	parameters, err := h.resolveSubscriptionParameters(reconciler.bus.GetSpec(), subscription.Spec)
	if err != nil {
		return err
	}
	channel := NewChannelReferenceFromSubscription(subscription)
	ref := NewSubscriptionReference(subscription)
	subscriptionCopy := subscription.DeepCopy()
	var cond *channelsv1alpha1.SubscriptionCondition
	err = h.SubscribeFunc(channel, ref, parameters)
	if err != nil {
		reconciler.RecordSubscriptionEventf(ref, corev1.EventTypeWarning, errResourceSync, "Error subscribing: %s", err)
		cond = util.NewSubscriptionCondition(channelsv1alpha1.SubscriptionDispatching, corev1.ConditionFalse, errResourceSync, err.Error())
	} else {
		reconciler.RecordSubscriptionEventf(ref, corev1.EventTypeNormal, successSynced, "Subscribed successfully")
		cond = util.NewSubscriptionCondition(channelsv1alpha1.SubscriptionDispatching, corev1.ConditionTrue, successSynced, "Subscription dispatcher successfully created")
	}
	util.SetSubscriptionCondition(&subscriptionCopy.Status, *cond)
	if !equality.Semantic.DeepEqual(subscription.Status, subscriptionCopy.Status) {
		// status has changed, update subscription
		_, errS := reconciler.eventingClient.ChannelsV1alpha1().Subscriptions(subscription.Namespace).Update(subscriptionCopy)
		if errS != nil {
			h.logger.Warnf("Could not update subscription status: %v", errS)
			if err != nil {
				return fmt.Errorf("error subscribing (%v); error updating subscription status (%v)", err, errS)
			}
			return errS
		}
	}
	return err
}

func (h EventHandlerFuncs) onUnsubscribe(subscription *channelsv1alpha1.Subscription, reconciler *Reconciler) error {
	if h.UnsubscribeFunc == nil {
		return nil
	}
	channel := NewChannelReferenceFromSubscription(subscription)
	ref := NewSubscriptionReference(subscription)
	if err := h.UnsubscribeFunc(channel, ref); err != nil {
		reconciler.RecordSubscriptionEventf(ref, corev1.EventTypeWarning, errResourceSync, "Error unsubscribing: %s", err)
		return err
	}
	reconciler.RecordSubscriptionEventf(ref, corev1.EventTypeNormal, successSynced, "Unsubscribed successfully")
	// skip updating status conditions since the subscription was deleted
	return nil
}

func (h EventHandlerFuncs) onReceiveMessage(channel ChannelReference, message *Message) error {
	if h.ReceiveMessageFunc == nil {
		// TODO use a static error
		return fmt.Errorf("unable to dispatch message")
	}
	return h.ReceiveMessageFunc(channel, message)
}

// resolveChannelParameters resolves the given Channel Parameters and the Bus'
// Channel Parameters, returning an ResolvedParameters or an Error.
func (h EventHandlerFuncs) resolveChannelParameters(bus *channelsv1alpha1.BusSpec, channel channelsv1alpha1.ChannelSpec) (ResolvedParameters, error) {
	genericBusParameters := bus.Parameters
	var parameters *[]channelsv1alpha1.Parameter
	if genericBusParameters != nil {
		parameters = genericBusParameters.Channel
	}
	return h.resolveParameters(parameters, channel.Arguments)
}

// resolveSubscriptionParameters resolves the given Subscription Parameters and
// the Bus' Subscription Parameters, returning an Attributes or an Error.
func (h EventHandlerFuncs) resolveSubscriptionParameters(bus *channelsv1alpha1.BusSpec, subscription channelsv1alpha1.SubscriptionSpec) (ResolvedParameters, error) {
	genericBusParameters := bus.Parameters
	var parameters *[]channelsv1alpha1.Parameter
	if genericBusParameters != nil {
		parameters = genericBusParameters.Subscription
	}
	return h.resolveParameters(parameters, subscription.Arguments)
}

// resolveParameters resolves a slice of Parameters and a slice of arguments and
// returns an Attributes or an error if there are missing Arguments. Each
// Parameter represents a variable that must be provided by an Argument or
// optionally defaulted if a default value for the Parameter is specified.
// resolveAttributes combines the given arrays of Parameters and Arguments,
// using default values where necessary and returning an error if there are
// missing Arguments.
func (h EventHandlerFuncs) resolveParameters(parameters *[]channelsv1alpha1.Parameter, arguments *[]channelsv1alpha1.Argument) (ResolvedParameters, error) {
	resolved := make(ResolvedParameters)
	known := make(map[string]interface{})
	required := make(map[string]interface{})

	// apply parameters
	if parameters != nil {
		for _, param := range *parameters {
			known[param.Name] = true
			if param.Default != nil {
				resolved[param.Name] = *param.Default
			} else {
				required[param.Name] = true
			}
		}
	}
	// apply arguments
	if arguments != nil {
		for _, arg := range *arguments {
			if _, ok := known[arg.Name]; !ok {
				// ignore arguments not defined by parameters
				h.logger.Warnf("Skipping unknown argument: %s", arg.Name)
				continue
			}
			delete(required, arg.Name)
			resolved[arg.Name] = arg.Value
		}
	}

	// check for missing arguments
	if len(required) != 0 {
		missing := []string{}
		for name := range required {
			missing = append(missing, name)
		}
		return nil, fmt.Errorf("missing required arguments: %v", missing)
	}

	return resolved, nil
}
