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
	cluster "github.com/bsm/sarama-cluster"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"github.com/knative/eventing/contrib/kafka/pkg/controller"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/provisioners/multichannelfanout"
	topicUtils "github.com/knative/eventing/pkg/provisioners/utils"
)

type KafkaDispatcher struct {
	// TODO: config doesn't have to be atomic as it is read and updated using updateLock.
	config           atomic.Value
	hostToChannelMap atomic.Value
	updateLock       sync.Mutex

	receiver   *provisioners.MessageReceiver
	dispatcher *provisioners.MessageDispatcher

	kafkaAsyncProducer sarama.AsyncProducer
	kafkaConsumers     map[provisioners.ChannelReference]map[subscription]KafkaConsumer
	kafkaCluster       KafkaCluster

	logger *zap.Logger
}

type KafkaConsumer interface {
	Messages() <-chan *sarama.ConsumerMessage
	Partitions() <-chan cluster.PartitionConsumer
	MarkOffset(msg *sarama.ConsumerMessage, metadata string)
	Close() (err error)
}

type KafkaCluster interface {
	NewConsumer(groupID string, topics []string) (KafkaConsumer, error)

	GetConsumerMode() cluster.ConsumerMode
}

type saramaCluster struct {
	kafkaBrokers []string

	consumerMode cluster.ConsumerMode
}

func (c *saramaCluster) NewConsumer(groupID string, topics []string) (KafkaConsumer, error) {
	consumerConfig := cluster.NewConfig()
	consumerConfig.Version = sarama.V1_1_0_0
	consumerConfig.Group.Mode = c.consumerMode
	return cluster.NewConsumer(c.kafkaBrokers, groupID, topics, consumerConfig)
}

func (c *saramaCluster) GetConsumerMode() cluster.ConsumerMode {
	return c.consumerMode
}

type subscription struct {
	UID           string
	SubscriberURI string
	ReplyURI      string
}

// configDiff diffs the new config with the existing config. If there are no differences, then the
// empty string is returned. If there are differences, then a non-empty string is returned
// describing the differences.
func (d *KafkaDispatcher) configDiff(updated *multichannelfanout.Config) string {
	return cmp.Diff(d.getConfig(), updated)
}

func (d *KafkaDispatcher) UpdateConfig(config *multichannelfanout.Config) error {
	if config == nil {
		return errors.New("nil config")
	}

	d.updateLock.Lock()
	defer d.updateLock.Unlock()

	if diff := d.configDiff(config); diff != "" {
		d.logger.Info("Updating config (-old +new)", zap.String("diff", diff))

		// Create hostToChannelMap before updating kafkaConsumers.
		// But update the map only after updating kafkaConsumers.
		hcMap, err := createHostToChannelMap(config)
		if err != nil {
			return err
		}

		newSubs := make(map[subscription]bool)

		// Subscribe to new subscriptions.
		// TODO: Error returned by subscribe/unsubscribe must be handled.
		// https://github.com/knative/eventing/issues/1072.
		for _, cc := range config.ChannelConfigs {
			channelRef := provisioners.ChannelReference{
				Name:      cc.Name,
				Namespace: cc.Namespace,
			}
			for _, subSpec := range cc.FanoutConfig.Subscriptions {
				sub := newSubscription(subSpec)
				if _, ok := d.kafkaConsumers[channelRef][sub]; !ok {
					// only subscribe when not exists in channel-subscriptions map
					// do not need to resubscribe every time channel fanout config is updated
					d.subscribe(channelRef, sub)
				}

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
		// At this point all updates are done and hostToChannelMap is created successfully.
		// Update the atomic value.
		d.setHostToChannelMap(hcMap)

		// Update the config so that it can be used for comparison during next sync
		d.setConfig(config)
	}
	return nil
}

func createHostToChannelMap(config *multichannelfanout.Config) (map[string]provisioners.ChannelReference, error) {
	hcMap := make(map[string]provisioners.ChannelReference, len(config.ChannelConfigs))
	for _, cConfig := range config.ChannelConfigs {
		if cr, ok := hcMap[cConfig.HostName]; ok {
			return nil, fmt.Errorf(
				"duplicate hostName found. Each channel must have a unique host header. HostName:%s, channel:%s.%s, channel:%s.%s",
				cConfig.HostName,
				cConfig.Namespace,
				cConfig.Name,
				cr.Namespace,
				cr.Name)
		}
		hcMap[cConfig.HostName] = provisioners.ChannelReference{Name: cConfig.Name, Namespace: cConfig.Namespace}
	}
	return hcMap, nil
}

// Start starts the kafka dispatcher's message processing.
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

	return d.receiver.Start(stopCh)
}

// subscribe reads kafkaConsumers which gets updated in UpdateConfig in a separate go-routine.
// subscribe must be called under updateLock.
func (d *KafkaDispatcher) subscribe(channelRef provisioners.ChannelReference, sub subscription) error {
	d.logger.Info("Subscribing", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))

	topicName := topicUtils.TopicName(controller.KafkaChannelSeparator, channelRef.Namespace, channelRef.Name)

	group := fmt.Sprintf("%s.%s", controller.Name, sub.UID)
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

	if cluster.ConsumerModePartitions == d.kafkaCluster.GetConsumerMode() {
		go d.partitionConsumerLoop(consumer, channelRef, sub)
	} else {
		go d.multiplexConsumerLoop(consumer, channelRef, sub)
	}
	return nil
}

func (d *KafkaDispatcher) partitionConsumerLoop(consumer KafkaConsumer, channelRef provisioners.ChannelReference, sub subscription) {
	d.logger.Info("Partition Consumer for subscription started", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))
	for {
		pc, more := <-consumer.Partitions()
		if !more {
			break
		}
		go func(pc cluster.PartitionConsumer) {
			for msg := range pc.Messages() {
				d.dispatch(channelRef, sub, consumer, msg)
			}
		}(pc)
	}
	d.logger.Info("Partition Consumer for subscription stopped", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))
}

func (d *KafkaDispatcher) multiplexConsumerLoop(consumer KafkaConsumer, channelRef provisioners.ChannelReference, sub subscription) {
	d.logger.Info("Consumer for subscription started", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))
	for {
		msg, more := <-consumer.Messages()
		if more {
			d.dispatch(channelRef, sub, consumer, msg)
		} else {
			break
		}
	}
	d.logger.Info("Consumer for subscription stopped", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))
}

func (d *KafkaDispatcher) dispatch(channelRef provisioners.ChannelReference, sub subscription, consumer KafkaConsumer,
	msg *sarama.ConsumerMessage) error {
	d.logger.Info("Dispatching a message for subscription", zap.Any("channelRef", channelRef),
		zap.Any("subscription", sub), zap.Any("partition", msg.Partition), zap.Any("offset", msg.Offset))
	message := fromKafkaMessage(msg)
	err := d.dispatchMessage(message, sub)
	if err != nil {
		d.logger.Warn("Got error trying to dispatch message", zap.Error(err))
	}
	// TODO: handle errors with pluggable strategy
	consumer.MarkOffset(msg, "") // Mark message as processed
	return err
}

// unsubscribe reads kafkaConsumers which gets updated in UpdateConfig in a separate go-routine.
// unsubscribe must be called under updateLock.
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

func (d *KafkaDispatcher) getHostToChannelMap() map[string]provisioners.ChannelReference {
	return d.hostToChannelMap.Load().(map[string]provisioners.ChannelReference)
}

func (d *KafkaDispatcher) setHostToChannelMap(hcMap map[string]provisioners.ChannelReference) {
	d.hostToChannelMap.Store(hcMap)
}

func NewDispatcher(brokers []string, consumerMode cluster.ConsumerMode, logger *zap.Logger) (*KafkaDispatcher, error) {
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

		kafkaCluster:       &saramaCluster{kafkaBrokers: brokers, consumerMode: consumerMode},
		kafkaConsumers:     make(map[provisioners.ChannelReference]map[subscription]KafkaConsumer),
		kafkaAsyncProducer: producer,

		logger: logger,
	}
	receiverFunc, err := provisioners.NewMessageReceiver(
		func(channel provisioners.ChannelReference, message *provisioners.Message) error {
			dispatcher.kafkaAsyncProducer.Input() <- toKafkaMessage(channel, message)
			return nil
		},
		logger.Sugar(),
		provisioners.ResolveChannelFromHostHeader(provisioners.ResolveChannelFromHostFunc(dispatcher.getChannelReferenceFromHost)))
	if err != nil {
		return nil, err
	}
	dispatcher.receiver = receiverFunc
	dispatcher.setConfig(&multichannelfanout.Config{})
	dispatcher.setHostToChannelMap(map[string]provisioners.ChannelReference{})
	return dispatcher, nil
}

func (d *KafkaDispatcher) getChannelReferenceFromHost(host string) (provisioners.ChannelReference, error) {
	chMap := d.getHostToChannelMap()
	cr, ok := chMap[host]
	if !ok {
		return cr, fmt.Errorf("invalid Hostname:%s. Hostname not found in ConfigMap for any Channel", host)
	}
	return cr, nil
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

func newSubscription(spec eventingduck.SubscriberSpec) subscription {
	return subscription{
		UID:           string(spec.UID),
		SubscriberURI: spec.SubscriberURI,
		ReplyURI:      spec.ReplyURI,
	}
}
