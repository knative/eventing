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

package gcppubsub

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/buses"
)

type CloudPubSubBus struct {
	busRef       buses.BusReference
	reconciler   *buses.Reconciler
	dispatcher   buses.BusDispatcher
	provisioner  buses.BusProvisioner
	pubsubClient *pubsub.Client
	receivers    map[string]context.CancelFunc
}

func NewCloudPubSubBusDispatcher(busRef buses.BusReference, projectID string, opts *buses.BusOpts) (*CloudPubSubBus, error) {
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	bus := &CloudPubSubBus{
		busRef:       busRef,
		pubsubClient: pubsubClient,
	}
	eventHandlers := buses.EventHandlerFuncs{
		SubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			return bus.startReceivingEvents(subscriptionRef, parameters)
		},
		UnsubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference) error {
			bus.stopReceivingEvents(subscriptionRef)
			return nil
		},
		ReceiveMessageFunc: func(channelRef buses.ChannelReference, messgae *buses.Message) error {
			return bus.sendEventToTopic(channelRef, messgae)
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(busRef, eventHandlers, opts)
	bus.reconciler = opts.Reconciler
	bus.receivers = make(map[string]context.CancelFunc)
	return bus, nil
}

func NewCloudPubSubBusProvisioner(busRef buses.BusReference, projectID string, opts *buses.BusOpts) (*CloudPubSubBus, error) {
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	bus := &CloudPubSubBus{
		busRef:       busRef,
		pubsubClient: pubsubClient,
	}
	eventHandlers := buses.EventHandlerFuncs{
		ProvisionFunc: func(channelRef buses.ChannelReference, parameters buses.ResolvedParameters) error {
			return bus.createTopic(channelRef, parameters)
		},
		UnprovisionFunc: func(channelRef buses.ChannelReference) error {
			return bus.deleteTopic(channelRef)
		},
		SubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			return bus.createOrUpdateSubscription(channelRef, subscriptionRef, parameters)
		},
		UnsubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference) error {
			return bus.deleteSubscription(subscriptionRef)
		},
	}
	bus.provisioner = buses.NewBusProvisioner(busRef, eventHandlers, opts)
	return bus, nil
}

func (b *CloudPubSubBus) Run(threadness int, stopCh <-chan struct{}) {
	if b.dispatcher != nil {
		b.dispatcher.Run(threadness, stopCh)
	}
	if b.provisioner != nil {
		b.provisioner.Run(threadness, stopCh)
	}
}

// startReceivingEvents will receive events from a Cloud Pub/Sub Subscription for a
// Knative Subscription. This method will not block, but will continue to
// receive events until either this method or StopReceivingEvents is called for
// the same Subscription.
func (b *CloudPubSubBus) startReceivingEvents(subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
	ctx := context.Background()
	cctx, cancel := context.WithCancel(ctx)

	// cancel current subscription receiver, if any
	b.stopReceivingEvents(subscriptionRef)

	subscriptionID := b.subscriptionID(subscriptionRef)
	subscription := b.pubsubClient.Subscription(subscriptionID)
	b.receivers[subscriptionID] = cancel

	// check if subscription exists before receiving
	if exists, err := subscription.Exists(ctx); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("cannot receive message for a non-existent subscription %s", subscriptionID)
	}

	// subscription.Receive blocks, so run it in a goroutine
	go func() {
		glog.Infof("Start receiving events for subscription %q\n", subscriptionID)
		err := subscription.Receive(cctx, func(ctx context.Context, pubsubMessage *pubsub.Message) {
			message := &buses.Message{
				Headers: pubsubMessage.Attributes,
				Payload: pubsubMessage.Data,
			}
			err := b.dispatcher.DispatchMessage(subscriptionRef, message)
			if err != nil {
				glog.Warningf("Unable to dispatch event %q to %q", pubsubMessage.ID, subscriptionRef.String())
				pubsubMessage.Nack()
			} else {
				glog.Infof("Dispatched event %q to %q", pubsubMessage.ID, subscriptionRef.String())
				pubsubMessage.Ack()
			}
		})
		if err != nil {
			glog.Errorf("Error receiving messesages for %q: %v\n", subscriptionID, err)
		}
		delete(b.receivers, subscriptionID)
		b.reconciler.RequeueSubscription(subscriptionRef)
	}()

	return nil
}

// stopReceivingEvents stops receiving events for a previous call to
// StartReceivingEvents. Calls for a Subscription that is not not actively receiving
// are ignored.
func (b *CloudPubSubBus) stopReceivingEvents(subscriptionRef buses.SubscriptionReference) {
	subscriptionID := b.subscriptionID(subscriptionRef)
	if cancel, ok := b.receivers[subscriptionID]; ok {
		glog.Infof("Stop receiving events for subscription %q\n", subscriptionID)
		cancel()
		delete(b.receivers, subscriptionID)
	}
}

// sendEventToTopic sends a message to the Cloud Pub/Sub Topic backing the
// Channel.
func (b *CloudPubSubBus) sendEventToTopic(channelRef buses.ChannelReference, message *buses.Message) error {
	ctx := context.Background()

	topicID := b.topicID(channelRef)
	topic := b.pubsubClient.Topic(topicID)

	result := topic.Publish(ctx, &pubsub.Message{
		Data:       message.Payload,
		Attributes: message.Headers,
	})
	id, err := result.Get(ctx)
	if err != nil {
		return err
	}

	// TODO allow topics to be reused between publish events, call .Stop after an idle period
	topic.Stop()

	glog.Infof("Published a message to %s; msg ID: %v\n", topicID, id)
	return nil
}

// createTopic creates a Topic in Cloud Pub/Sub for the Channel.
func (b *CloudPubSubBus) createTopic(channelRef buses.ChannelReference, parameters buses.ResolvedParameters) error {
	ctx := context.Background()

	topicID := b.topicID(channelRef)
	topic := b.pubsubClient.Topic(topicID)

	// check if topic exists before creating
	if exists, err := topic.Exists(ctx); err != nil {
		return err
	} else if exists {
		return nil
	}

	glog.Infof("Create topic %q\n", topicID)
	topic, err := b.pubsubClient.CreateTopic(ctx, topicID)
	if err != nil {
		return err
	}

	return nil
}

// deleteTopic deletes the Topic in Cloud Pub/Sub for the Channel.
func (b *CloudPubSubBus) deleteTopic(channelRef buses.ChannelReference) error {
	ctx := context.Background()

	topicID := b.topicID(channelRef)
	topic := b.pubsubClient.Topic(topicID)

	// check if topic exists before deleting
	if exists, err := topic.Exists(ctx); err != nil {
		return err
	} else if !exists {
		return nil
	}

	glog.Infof("Delete topic %q\n", topicID)
	return topic.Delete(ctx)
}

// createOrUpdateSubscription creates a Subscription in Cloud Pub/Sub for the
// Knative Subscription, or idempotently updates a Subscription if it already
// exists.
func (b *CloudPubSubBus) createOrUpdateSubscription(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
	ctx := context.Background()

	subscriptionID := b.subscriptionID(subscriptionRef)
	subscription := b.pubsubClient.Subscription(subscriptionID)

	// check if subscription exists before creating
	if exists, err := subscription.Exists(ctx); err != nil {
		return err
	} else if exists {
		// TODO once the bus has configurable params, update subscription configuration
		// _, err := subscription.Update(b.ctx, pubsub.SubscriptionConfigToUpdate{})
		// return err
		return nil
	}

	// create subscription
	topicID := b.topicID(channelRef)
	topic := b.pubsubClient.Topic(topicID)
	glog.Infof("Create subscription %q for topic %q\n", subscriptionID, topicID)
	subscription, err := b.pubsubClient.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
		Topic: topic,
	})
	return err
}

// deleteSubscription removes a Subscription from Cloud Pub/Sub for a Knative
// Subscription.
func (b *CloudPubSubBus) deleteSubscription(subscriptionRef buses.SubscriptionReference) error {
	ctx := context.Background()

	subscriptionID := b.subscriptionID(subscriptionRef)
	subscription := b.pubsubClient.Subscription(subscriptionID)

	// check if subscription exists before deleting
	if exists, err := subscription.Exists(ctx); err != nil {
		return err
	} else if !exists {
		return nil
	}

	glog.Infof("Deleting subscription %q\n", subscriptionID)
	return subscription.Delete(ctx)
}

func (b *CloudPubSubBus) topicID(channelRef buses.ChannelReference) string {
	return fmt.Sprintf("channel-%s-%s-%s", b.busRef.Name, channelRef.Namespace, channelRef.Name)
}

func (b *CloudPubSubBus) subscriptionID(subscriptionRef buses.SubscriptionReference) string {
	return fmt.Sprintf("subscription-%s-%s-%s", b.busRef.Name, subscriptionRef.Namespace, subscriptionRef.Name)
}
