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
	"github.com/knative/eventing/pkg/buses"
	"go.uber.org/zap"
)

// BusType is the type of the gcppubsub bus
const BusType = "gcppubsub"

type CloudPubSubBus struct {
	ref         buses.BusReference
	reconciler  *buses.Reconciler
	dispatcher  buses.BusDispatcher
	provisioner buses.BusProvisioner

	pubsubClient *pubsub.Client
	receivers    map[string]context.CancelFunc

	logger *zap.SugaredLogger
}

func NewCloudPubSubBusDispatcher(ref buses.BusReference, projectID string, opts *buses.BusOpts) (*CloudPubSubBus, error) {
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	bus := &CloudPubSubBus{
		ref:          ref,
		pubsubClient: pubsubClient,
	}
	eventHandlers := buses.EventHandlerFuncs{
		SubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			return bus.startReceivingEvents(subscription, parameters)
		},
		UnsubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference) error {
			bus.stopReceivingEvents(subscription)
			return nil
		},
		ReceiveMessageFunc: func(channel buses.ChannelReference, messgae *buses.Message) error {
			return bus.sendEventToTopic(channel, messgae)
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(ref, eventHandlers, opts)
	bus.logger = opts.Logger
	bus.reconciler = opts.Reconciler
	bus.receivers = make(map[string]context.CancelFunc)
	return bus, nil
}

func NewCloudPubSubBusProvisioner(ref buses.BusReference, projectID string, opts *buses.BusOpts) (*CloudPubSubBus, error) {
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	bus := &CloudPubSubBus{
		ref:          ref,
		pubsubClient: pubsubClient,
	}
	eventHandlers := buses.EventHandlerFuncs{
		ProvisionFunc: func(channel buses.ChannelReference, parameters buses.ResolvedParameters) error {
			return bus.createTopic(channel, parameters)
		},
		UnprovisionFunc: func(channel buses.ChannelReference) error {
			return bus.deleteTopic(channel)
		},
		SubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			return bus.createOrUpdateSubscription(channel, subscription, parameters)
		},
		UnsubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference) error {
			return bus.deleteSubscription(subscription)
		},
	}
	bus.provisioner = buses.NewBusProvisioner(ref, eventHandlers, opts)
	bus.logger = opts.Logger
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
func (b *CloudPubSubBus) startReceivingEvents(ref buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
	ctx := context.Background()
	cctx, cancel := context.WithCancel(ctx)

	// cancel current subscription receiver, if any
	b.stopReceivingEvents(ref)

	subscriptionID := b.subscriptionID(ref)
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
		b.logger.Infof("Start receiving events for subscription %q", subscriptionID)
		err := subscription.Receive(cctx, func(ctx context.Context, pubsubMessage *pubsub.Message) {
			message := &buses.Message{
				Headers: pubsubMessage.Attributes,
				Payload: pubsubMessage.Data,
			}
			err := b.dispatcher.DispatchMessage(ref, message)
			if err != nil {
				b.logger.Warnf("Unable to dispatch event %q to %q", pubsubMessage.ID, ref.String())
				pubsubMessage.Nack()
			} else {
				b.logger.Infof("Dispatched event %q to %q", pubsubMessage.ID, ref.String())
				pubsubMessage.Ack()
			}
		})
		if err != nil {
			b.logger.Errorf("Error receiving messesages for %q: %v", subscriptionID, err)
		}
		delete(b.receivers, subscriptionID)
		b.reconciler.RequeueSubscription(ref)
	}()

	return nil
}

// stopReceivingEvents stops receiving events for a previous call to
// StartReceivingEvents. Calls for a Subscription that is not not actively receiving
// are ignored.
func (b *CloudPubSubBus) stopReceivingEvents(ref buses.SubscriptionReference) {
	subscriptionID := b.subscriptionID(ref)
	if cancel, ok := b.receivers[subscriptionID]; ok {
		b.logger.Infof("Stop receiving events for subscription %q", subscriptionID)
		cancel()
		delete(b.receivers, subscriptionID)
	}
}

// sendEventToTopic sends a message to the Cloud Pub/Sub Topic backing the
// Channel.
func (b *CloudPubSubBus) sendEventToTopic(channel buses.ChannelReference, message *buses.Message) error {
	ctx := context.Background()

	topicID := b.topicID(channel)
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

	b.logger.Infof("Published a message to %s; msg ID: %v", topicID, id)
	return nil
}

// createTopic creates a Topic in Cloud Pub/Sub for the Channel.
func (b *CloudPubSubBus) createTopic(channel buses.ChannelReference, parameters buses.ResolvedParameters) error {
	ctx := context.Background()

	topicID := b.topicID(channel)
	topic := b.pubsubClient.Topic(topicID)

	// check if topic exists before creating
	if exists, err := topic.Exists(ctx); err != nil {
		return err
	} else if exists {
		return nil
	}

	b.logger.Infof("Create topic %q", topicID)
	topic, err := b.pubsubClient.CreateTopic(ctx, topicID)
	if err != nil {
		return err
	}

	return nil
}

// deleteTopic deletes the Topic in Cloud Pub/Sub for the Channel.
func (b *CloudPubSubBus) deleteTopic(channel buses.ChannelReference) error {
	ctx := context.Background()

	topicID := b.topicID(channel)
	topic := b.pubsubClient.Topic(topicID)

	// check if topic exists before deleting
	if exists, err := topic.Exists(ctx); err != nil {
		return err
	} else if !exists {
		return nil
	}

	b.logger.Infof("Delete topic %q", topicID)
	return topic.Delete(ctx)
}

// createOrUpdateSubscription creates a Subscription in Cloud Pub/Sub for the
// Knative Subscription, or idempotently updates a Subscription if it already
// exists.
func (b *CloudPubSubBus) createOrUpdateSubscription(channel buses.ChannelReference, ref buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
	ctx := context.Background()

	subscriptionID := b.subscriptionID(ref)
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
	topicID := b.topicID(channel)
	topic := b.pubsubClient.Topic(topicID)
	b.logger.Infof("Create subscription %q for topic %q", subscriptionID, topicID)
	subscription, err := b.pubsubClient.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
		Topic: topic,
	})
	return err
}

// deleteSubscription removes a Subscription from Cloud Pub/Sub for a Knative
// Subscription.
func (b *CloudPubSubBus) deleteSubscription(ref buses.SubscriptionReference) error {
	ctx := context.Background()

	subscriptionID := b.subscriptionID(ref)
	subscription := b.pubsubClient.Subscription(subscriptionID)

	// check if subscription exists before deleting
	if exists, err := subscription.Exists(ctx); err != nil {
		return err
	} else if !exists {
		return nil
	}

	b.logger.Infof("Deleting subscription %q", subscriptionID)
	return subscription.Delete(ctx)
}

func (b *CloudPubSubBus) topicID(channel buses.ChannelReference) string {
	return fmt.Sprintf("channel-%s-%s-%s", b.ref.Name, channel.Namespace, channel.Name)
}

func (b *CloudPubSubBus) subscriptionID(subscription buses.SubscriptionReference) string {
	return fmt.Sprintf("subscription-%s-%s-%s", b.ref.Name, subscription.Namespace, subscription.Name)
}
