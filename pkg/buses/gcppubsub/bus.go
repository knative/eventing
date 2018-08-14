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
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
)

type PubSubBus struct {
	name              string
	monitor           *buses.Monitor
	messageReceiver   *buses.MessageReceiver
	messageDispatcher *buses.MessageDispatcher
	pubsubClient      *pubsub.Client
	receivers         map[string]context.CancelFunc
}

func (b *PubSubBus) CreateTopic(channel *channelsv1alpha1.Channel, parameters buses.ResolvedParameters) error {
	ctx := context.Background()

	topicID := b.topicID(channel)
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

func (b *PubSubBus) DeleteTopic(channel *channelsv1alpha1.Channel) error {
	ctx := context.Background()

	topicID := b.topicID(channel)
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

func (b *PubSubBus) CreateOrUpdateSubscription(sub *channelsv1alpha1.Subscription, parameters buses.ResolvedParameters) error {
	ctx := context.Background()

	subscriptionID := b.subscriptionID(sub)
	subscription := b.pubsubClient.Subscription(subscriptionID)

	// check if subscription exists before creating
	if exists, err := subscription.Exists(ctx); err != nil {
		return err
	} else if exists {
		// TODO update subscription configuration
		// _, err := subscription.Update(b.ctx, pubsub.SubscriptionConfigToUpdate{})
		// return err
		return nil
	}

	// create subscription
	channel := b.monitor.Channel(sub.Spec.Channel, sub.Namespace)
	if channel == nil {
		return fmt.Errorf("Cannot create a Subscription for unknown Channel %q", sub.Spec.Channel)
	}
	topicID := b.topicID(channel)
	topic := b.pubsubClient.Topic(topicID)
	glog.Infof("Create subscription %q for topic %q\n", subscriptionID, topicID)
	subscription, err := b.pubsubClient.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
		Topic: topic,
	})
	return err
}

func (b *PubSubBus) DeleteSubscription(sub *channelsv1alpha1.Subscription) error {
	ctx := context.Background()

	subscriptionID := b.subscriptionID(sub)
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

func (b *PubSubBus) SendEventToTopic(channel *channelsv1alpha1.Channel, message *buses.Message) error {
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

	glog.Infof("Published a message to %s; msg ID: %v\n", topicID, id)
	return nil
}

func (b *PubSubBus) ReceiveEvents(sub *channelsv1alpha1.Subscription, parameters buses.ResolvedParameters) error {
	ctx := context.Background()
	cctx, cancel := context.WithCancel(ctx)

	// cancel current subscription receiver, if any
	b.StopReceiveEvents(sub)

	subscriptionID := b.subscriptionID(sub)
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
			subscriber := sub.Spec.Subscriber
			message := &buses.Message{
				Headers: pubsubMessage.Attributes,
				Payload: pubsubMessage.Data,
			}
			defaults := buses.DispatchDefaults{
				Namespace: sub.Namespace,
				ReplyTo:   sub.Spec.ReplyTo,
			}
			err := b.messageDispatcher.DispatchMessage(message, subscriber, defaults)
			if err != nil {
				glog.Warningf("Unable to dispatch event %q to %q", pubsubMessage.ID, subscriber)
				pubsubMessage.Nack()
			} else {
				glog.Infof("Dispatched event %q to %q", pubsubMessage.ID, subscriber)
				pubsubMessage.Ack()
			}
		})
		if err != nil {
			glog.Errorf("Error receiving messesages for %q: %v\n", subscriptionID, err)
		}
		delete(b.receivers, subscriptionID)
		b.monitor.RequeueSubscription(sub)
	}()

	return nil
}

func (b *PubSubBus) StopReceiveEvents(subscription *channelsv1alpha1.Subscription) error {
	subscriptionID := b.subscriptionID(subscription)
	if cancel, ok := b.receivers[subscriptionID]; ok {
		glog.Infof("Stop receiving events for subscription %q\n", subscriptionID)
		cancel()
		delete(b.receivers, subscriptionID)
	}
	return nil
}

func (b *PubSubBus) topicID(channel *channelsv1alpha1.Channel) string {
	return fmt.Sprintf("channel-%s-%s-%s", b.name, channel.Namespace, channel.Name)
}

func (b *PubSubBus) subscriptionID(subscription *channelsv1alpha1.Subscription) string {
	return fmt.Sprintf("subscription-%s-%s-%s", b.name, subscription.Namespace, subscription.Name)
}

func (b *PubSubBus) ReceiveMessage(channel *buses.ChannelReference, message *buses.Message) error {
	c := b.monitor.Channel(channel.Name, channel.Namespace)
	if c == nil {
		return buses.ErrUnknownChannel
	}

	err := b.SendEventToTopic(c, message)
	if err != nil {
		return fmt.Errorf("unable to send event to topic %q: %v", channel.Name, err)
	}

	return nil
}

func (b *PubSubBus) splitChannelName(host string) (string, string) {
	chunks := strings.Split(host, ".")
	channel := chunks[0]
	namespace := chunks[1]
	return channel, namespace
}

func NewPubSubBus(
	name string, projectID string,
	monitor *buses.Monitor,
	messageReceiver *buses.MessageReceiver,
	messageDispatcher *buses.MessageDispatcher,
) (*PubSubBus, error) {
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	bus := PubSubBus{
		name:              name,
		monitor:           monitor,
		messageReceiver:   messageReceiver,
		messageDispatcher: messageDispatcher,
		pubsubClient:      pubsubClient,
		receivers:         map[string]context.CancelFunc{},
	}

	return &bus, nil
}
