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
package dispatcher

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/provisioners/kafka/controller"
	topicUtils "github.com/knative/eventing/pkg/provisioners/utils"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
)

type KafkaDispatcher struct {
	config     atomic.Value
	updateLock sync.Mutex

	receiver   *provisioners.MessageReceiver
	dispatcher *provisioners.MessageDispatcher

	kafkaAsyncProducer sarama.AsyncProducer
	kafkaConsumers     map[provisioners.ChannelReference]map[subscription]KafkaConsumer
	kafkaCluster       KafkaCluster

	logger *zap.Logger
}

type KafkaConsumer interface {
	Messages() <-chan *sarama.ConsumerMessage
	MarkOffset(msg *sarama.ConsumerMessage, metadata string)
	Close() (err error)
}

type KafkaCluster interface {
	NewConsumer(groupID string, topics []string) (KafkaConsumer, error)
}

type saramaCluster struct {
	kafkaBrokers []string
}

func (c *saramaCluster) NewConsumer(groupID string, topics []string) (KafkaConsumer, error) {
	consumerConfig := cluster.NewConfig()
	consumerConfig.Version = sarama.V1_1_0_0
	return cluster.NewConsumer(c.kafkaBrokers, groupID, topics, consumerConfig)
}

type subscription struct {
	Namespace     string
	Name          string
	SubscriberURI string
	ReplyURI      string
}

// ConfigDiffs diffs the new config with the existing config. If there are no differences, then the
// empty string is returned. If there are differences, then a non-empty string is returned
// describing the differences.
func (d *KafkaDispatcher) ConfigDiff(updated *multichannelfanout.Config) string {
	return cmp.Diff(d.getConfig(), updated)
}

func (d *KafkaDispatcher) UpdateConfig(config *multichannelfanout.Config) error {
	if config == nil {
		return errors.New("nil config")
	}

	d.updateLock.Lock()
	defer d.updateLock.Unlock()

	if diff := d.ConfigDiff(config); diff != "" {
		d.logger.Info("Updating config (-old +new)", zap.String("diff", diff))

		newSubs := make(map[subscription]bool)

		// Subscribe to new subscriptions
		for _, cc := range config.ChannelConfigs {
			channelRef := provisioners.ChannelReference{
				Name:      cc.Name,
				Namespace: cc.Namespace,
			}
			for _, subSpec := range cc.FanoutConfig.Subscriptions {
				sub := newSubscription(subSpec)
				if _, ok := d.kafkaConsumers[channelRef][sub]; ok {
					// subscribe can be called multiple times for the same subscription,
					// unsubscribe before we resubscribe.
					// TODO The behavior to unsubscribe and re-subscribe is retained from the older kafka bus
					// implementation. It is not clear as to why this is needed instead of just re-using the same
					// consumer.
					err := d.unsubscribe(channelRef, sub)
					if err != nil {
						return err
					}
				}
				d.subscribe(channelRef, sub)
				newSubs[sub] = true
			}
		}

		// Unsubscribe and close consumer for any deleted subscriptions
		for channelRef, subMap := range d.kafkaConsumers {
			for sub := range subMap {
				if ok := newSubs[sub]; !ok {
					d.unsubscribe(channelRef, sub)
				}
			}
		}

		// Update the config so that it can be used for comparison during next sync
		d.setConfig(config)
	}
	return nil
}

// Run starts the kafka dispatcher's message processing.
func (d *KafkaDispatcher) Start(stopCh <-chan struct{}) error {
	if d.receiver == nil {
		return fmt.Errorf("message receiver is not set")
	}

	if d.kafkaAsyncProducer == nil {
		return fmt.Errorf("kafkaAsyncProducer is not set")
	}

	go func() {
		for {
			select {
			case e := <-d.kafkaAsyncProducer.Errors():
				d.logger.Warn("Got", zap.Error(e))
			case s := <-d.kafkaAsyncProducer.Successes():
				d.logger.Info("Sent", zap.Any("success", s))
			case <-stopCh:
				return
			}
		}
	}()

	d.receiver.Run(stopCh)

	return nil
}

func (d *KafkaDispatcher) subscribe(channelRef provisioners.ChannelReference, sub subscription) error {

	d.logger.Info("Subscribing", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))

	topicName := topicUtils.TopicName(controller.KafkaChannelSeparator, channelRef.Namespace, channelRef.Name)

	group := fmt.Sprintf("%s.%s.%s", controller.Name, sub.Namespace, sub.Name)
	consumer, err := d.kafkaCluster.NewConsumer(group, []string{topicName})
	if err != nil {
		// we can not create a consumer - logging that, with reason
		d.logger.Info("Could not create proper consumer", zap.Error(err))
		return err
	}

	channelMap, ok := d.kafkaConsumers[channelRef]
	if !ok {
		channelMap = make(map[subscription]KafkaConsumer)
		d.kafkaConsumers[channelRef] = channelMap
	}
	channelMap[sub] = consumer

	go func() {
		for {
			msg, more := <-consumer.Messages()
			if more {
				d.logger.Info("Dispatching a message for subscription", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))
				message := fromKafkaMessage(msg)
				err := d.dispatchMessage(message, sub)
				if err != nil {
					d.logger.Warn("Got error trying to dispatch message", zap.Error(err))
				}
				// TODO: handle errors with pluggable strategy
				consumer.MarkOffset(msg, "") // Mark message as processed
			} else {
				break
			}
		}
		d.logger.Info("Consumer for subscription stopped", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))
	}()

	return nil
}

func (d *KafkaDispatcher) unsubscribe(channel provisioners.ChannelReference, sub subscription) error {
	d.logger.Info("Unsubscribing from channel", zap.Any("channel", channel), zap.Any("subscription", sub))
	if consumer, ok := d.kafkaConsumers[channel][sub]; ok {
		delete(d.kafkaConsumers[channel], sub)
		return consumer.Close()
	}
	return nil
}

// dispatchMessage sends the request to exactly one subscription. It handles both the `call` and
// the `sink` portions of the subscription.
func (d *KafkaDispatcher) dispatchMessage(m *provisioners.Message, sub subscription) error {
	return d.dispatcher.DispatchMessage(m, sub.SubscriberURI, sub.ReplyURI, provisioners.DispatchDefaults{})
}

func (d *KafkaDispatcher) getConfig() *multichannelfanout.Config {
	return d.config.Load().(*multichannelfanout.Config)
}

func (d *KafkaDispatcher) setConfig(config *multichannelfanout.Config) {
	d.config.Store(config)
}

func NewDispatcher(brokers []string, logger *zap.Logger) (*KafkaDispatcher, error) {

	conf := sarama.NewConfig()
	conf.Version = sarama.V1_1_0_0
	conf.ClientID = controller.Name + "-dispatcher"
	client, err := sarama.NewClient(brokers, conf)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka client: %v", err)
	}

	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka producer: %v", err)
	}

	dispatcher := &KafkaDispatcher{
		dispatcher: provisioners.NewMessageDispatcher(logger.Sugar()),

		kafkaCluster:       &saramaCluster{kafkaBrokers: brokers},
		kafkaConsumers:     make(map[provisioners.ChannelReference]map[subscription]KafkaConsumer),
		kafkaAsyncProducer: producer,

		logger: logger,
	}
	receiverFunc := provisioners.NewMessageReceiver(
		func(channel provisioners.ChannelReference, message *provisioners.Message) error {
			dispatcher.kafkaAsyncProducer.Input() <- toKafkaMessage(channel, message)
			return nil
		}, logger.Sugar())
	dispatcher.receiver = receiverFunc
	dispatcher.setConfig(&multichannelfanout.Config{})
	return dispatcher, nil
}

func fromKafkaMessage(kafkaMessage *sarama.ConsumerMessage) *provisioners.Message {
	headers := make(map[string]string)
	for _, header := range kafkaMessage.Headers {
		headers[string(header.Key)] = string(header.Value)
	}
	message := provisioners.Message{
		Headers: headers,
		Payload: kafkaMessage.Value,
	}
	return &message
}

func toKafkaMessage(channel provisioners.ChannelReference, message *provisioners.Message) *sarama.ProducerMessage {
	kafkaMessage := sarama.ProducerMessage{
		Topic: topicUtils.TopicName(controller.KafkaChannelSeparator, channel.Namespace, channel.Name),
		Value: sarama.ByteEncoder(message.Payload),
	}
	for h, v := range message.Headers {
		kafkaMessage.Headers = append(kafkaMessage.Headers, sarama.RecordHeader{
			Key:   []byte(h),
			Value: []byte(v),
		})
	}
	return &kafkaMessage
}

func newSubscription(spec eventingduck.ChannelSubscriberSpec) subscription {
	return subscription{
		Name:          spec.Ref.Name,
		Namespace:     spec.Ref.Namespace,
		SubscriberURI: spec.SubscriberURI,
		ReplyURI:      spec.ReplyURI,
	}
}
