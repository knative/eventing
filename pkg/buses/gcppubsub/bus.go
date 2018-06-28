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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
)

type PubSubBus struct {
	name           string
	monitor        *buses.Monitor
	pubsubClient   *pubsub.Client
	client         *http.Client
	forwardHeaders []string
	receivers      map[string]context.CancelFunc
}

func (b *PubSubBus) CreateTopic(channel *channelsv1alpha1.Channel, attributes buses.Attributes) error {
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

func (b *PubSubBus) CreateOrUpdateSubscription(sub *channelsv1alpha1.Subscription, attributes buses.Attributes) error {
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

func (b *PubSubBus) SendEventToTopic(channel *channelsv1alpha1.Channel, data []byte, attributes map[string]string) error {
	ctx := context.Background()

	topicID := b.topicID(channel)
	topic := b.pubsubClient.Topic(topicID)

	result := topic.Publish(ctx, &pubsub.Message{
		Data:       data,
		Attributes: attributes,
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

func (b *PubSubBus) ReceiveEvents(sub *channelsv1alpha1.Subscription, attributes buses.Attributes) error {
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
		err := subscription.Receive(cctx, func(ctx context.Context, message *pubsub.Message) {
			err := b.DispatchHTTPEvent(sub, message.Data, message.Attributes)
			if err != nil {
				glog.Warningf("Unable to dispatch event %q to %q", message.ID, sub.Spec.Subscriber)
				message.Nack()
			} else {
				glog.Infof("Dispatched event %q to %q", message.ID, sub.Spec.Subscriber)
				message.Ack()
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

func (b *PubSubBus) ReceiveHTTPEvent(res http.ResponseWriter, req *http.Request) {
	host := req.Host
	glog.Infof("Received request for %s\n", host)

	name, namespace := b.splitChannelName(host)
	channel := b.monitor.Channel(name, namespace)
	if channel == nil {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	attributes := b.headersToAttributes(b.safeHeaders(req.Header))

	err = b.SendEventToTopic(channel, data, attributes)
	if err != nil {
		glog.Warningf("Unable to send event to topic %q: %v", channel.Name, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusAccepted)
}

func (b *PubSubBus) DispatchHTTPEvent(subscription *channelsv1alpha1.Subscription, data []byte, attributes map[string]string) error {
	subscriber := subscription.Spec.Subscriber
	url := url.URL{
		Scheme: "http",
		Host:   subscription.Spec.Subscriber,
		Path:   "/",
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header = b.safeHeaders(b.attributesToHeaders(attributes))
	res, err := b.client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Subscribing service %q did not accept event: got HTTP %d", subscriber, res.StatusCode)
	}
	return nil
}

func (b *PubSubBus) splitChannelName(host string) (string, string) {
	chunks := strings.Split(host, ".")
	channel := chunks[0]
	namespace := chunks[1]
	return channel, namespace
}

func (b *PubSubBus) safeHeaders(raw http.Header) http.Header {
	safe := http.Header{}
	for _, header := range b.forwardHeaders {
		if value := raw.Get(header); value != "" {
			safe.Set(header, value)
		}
	}
	return safe
}

func (b *PubSubBus) headersToAttributes(headers http.Header) map[string]string {
	attributes := make(map[string]string)
	for name, value := range headers {
		// TODO hanle compound headers
		attributes[name] = value[0]
	}
	return attributes
}

func (b *PubSubBus) attributesToHeaders(attributes map[string]string) http.Header {
	headers := http.Header{}
	for name, value := range attributes {
		headers.Set(name, value)
	}
	return headers
}

func NewPubSubBus(name string, projectID string, monitor *buses.Monitor) (*PubSubBus, error) {
	forwardHeaders := []string{
		"content-type",
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
		"x-ot-span-context",
	}

	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	bus := PubSubBus{
		name:           name,
		monitor:        monitor,
		pubsubClient:   pubsubClient,
		client:         &http.Client{},
		forwardHeaders: forwardHeaders,
		receivers:      map[string]context.CancelFunc{},
	}

	return &bus, nil
}
