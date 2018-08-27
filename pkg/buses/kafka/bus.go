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

package kafka

import (
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/knative/eventing/pkg/buses"
)

const (
	// BusType is the type of the kafka bus
	BusType = "kafka"

	initialOffset = "initialOffset"
	numPartitions = "NumPartitions"
	newest        = "Newest"
	oldest        = "Oldest"
)

type KafkaBus struct {
	ref         buses.BusReference
	dispatcher  buses.BusDispatcher
	provisioner buses.BusProvisioner

	kafkaBrokers       []string
	kafkaClusterAdmin  sarama.ClusterAdmin
	kafkaAsyncProducer sarama.AsyncProducer
	kafkaConsumers     map[buses.SubscriptionReference]*cluster.Consumer

	logger *zap.SugaredLogger
}

func NewKafkaBusDispatcher(ref buses.BusReference, brokers []string, opts *buses.BusOpts) (*KafkaBus, error) {
	bus := &KafkaBus{
		ref:          ref,
		kafkaBrokers: brokers,
	}
	eventHandlers := buses.EventHandlerFuncs{
		SubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			return bus.subscribe(channel, subscription, parameters)
		},
		UnsubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference) error {
			return bus.unsubscribe(channel, subscription)
		},
		ReceiveMessageFunc: func(channel buses.ChannelReference, message *buses.Message) error {
			bus.kafkaAsyncProducer.Input() <- toKafkaMessage(channel, message)
			return nil
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(ref, eventHandlers, opts)
	bus.logger = opts.Logger
	bus.kafkaConsumers = make(map[buses.SubscriptionReference]*cluster.Consumer)

	conf := sarama.NewConfig()
	conf.Version = sarama.V1_1_0_0
	conf.ClientID = ref.Name + "-dispatcher"
	client, err := sarama.NewClient(brokers, conf)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka client: %v", err)
	}
	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka producer: %v", err)
	}
	bus.kafkaAsyncProducer = producer

	return bus, nil
}

func NewKafkaBusProvisioner(ref buses.BusReference, brokers []string, opts *buses.BusOpts) (*KafkaBus, error) {
	bus := &KafkaBus{
		ref:          ref,
		kafkaBrokers: brokers,
	}
	eventHandlers := buses.EventHandlerFuncs{
		ProvisionFunc: func(channel buses.ChannelReference, parameters buses.ResolvedParameters) error {
			return bus.provision(channel, parameters)
		},
		UnprovisionFunc: func(channel buses.ChannelReference) error {
			return bus.unprovision(channel)
		},
	}
	bus.provisioner = buses.NewBusProvisioner(ref, eventHandlers, opts)
	bus.logger = opts.Logger

	conf := sarama.NewConfig()
	conf.Version = sarama.V1_1_0_0
	conf.ClientID = ref.Name + "-provisioner"

	clusterAdmin, err := sarama.NewClusterAdmin(brokers, conf)
	if err != nil {
		return nil, fmt.Errorf("unable to building kafka admin client: %v", err)
	}
	bus.kafkaClusterAdmin = clusterAdmin

	return bus, nil
}

func (b *KafkaBus) Run(threadness int, stopCh <-chan struct{}) {
	if b.kafkaAsyncProducer != nil {
		go func() {
			for {
				select {
				case e := <-b.kafkaAsyncProducer.Errors():
					b.logger.Warnf("Got %v", e)
				case s := <-b.kafkaAsyncProducer.Successes():
					b.logger.Infof("Sent %v", s)
				case <-stopCh:
					return
				}
			}
		}()
	}
	if b.dispatcher != nil {
		b.dispatcher.Run(threadness, stopCh)
	}
	if b.provisioner != nil {
		b.provisioner.Run(threadness, stopCh)
	}
}

func (b *KafkaBus) subscribe(channel buses.ChannelReference, subscription buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
	if _, ok := b.kafkaConsumers[subscription]; ok {
		// subscribe can be called multiple times for the same subscription,
		//unsubscribe before we resubscribe
		err := b.unsubscribe(channel, subscription)
		if err != nil {
			return err
		}
	}

	b.logger.Infof("Subscribing %q to %q (%v)", subscription.String(), channel.String(), parameters)

	topicName := topicName(channel)

	initialOffset, err := resolveInitialOffset(parameters)
	if err != nil {
		return err
	}

	group := fmt.Sprintf("%s.%s.%s", b.ref.Name, subscription.Namespace, subscription.Name)
	consumerConfig := cluster.NewConfig()
	consumerConfig.Version = sarama.V1_1_0_0
	consumerConfig.Consumer.Offsets.Initial = initialOffset
	consumer, err := cluster.NewConsumer(b.kafkaBrokers, group, []string{topicName}, consumerConfig)
	if err != nil {
		return err
	}

	b.kafkaConsumers[subscription] = consumer

	go func() {
		for {
			msg, more := <-consumer.Messages()
			if more {
				b.logger.Infof("Dispatching a message for subscription %q", subscription.String())
				message := fromKafkaMessage(msg)
				err := b.dispatcher.DispatchMessage(subscription, message)
				if err != nil {
					b.logger.Warnf("Got error trying to dispatch message: %v", err)
				}
				// TODO: handle errors with pluggable strategy
				consumer.MarkOffset(msg, "") // Mark message as processed
			} else {
				break
			}
		}
		b.logger.Infof("Consumer for subscription %q stopped", subscription.String())
	}()

	return nil
}

func (b *KafkaBus) unsubscribe(channel buses.ChannelReference, subscription buses.SubscriptionReference) error {
	b.logger.Infof("Un-Subscribing %q from %q", subscription.String(), channel.String())
	if consumer, ok := b.kafkaConsumers[subscription]; ok {
		delete(b.kafkaConsumers, subscription)
		return consumer.Close()
	}
	return nil
}

func (b *KafkaBus) provision(channel buses.ChannelReference, parameters buses.ResolvedParameters) error {
	topicName := topicName(channel)
	b.logger.Infof("Provisioning topic %s on bus backed by Kafka", topicName)

	partitions := 1
	if p, ok := parameters[numPartitions]; ok {
		var err error
		partitions, err = strconv.Atoi(p)
		if err != nil {
			b.logger.Warnf("Could not parse partition count for %q: %s", channel.String(), p)
		}
	}

	err := b.kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
		ReplicationFactor: 1,
		NumPartitions:     int32(partitions),
	}, false)
	if err == sarama.ErrTopicAlreadyExists {
		return nil
	} else if err != nil {
		b.logger.Errorf("Error creating topic %s: %v", topicName, err)
	} else {
		b.logger.Infof("Successfully created topic %s", topicName)
	}
	return err
}

func (b *KafkaBus) unprovision(channel buses.ChannelReference) error {
	topicName := topicName(channel)
	b.logger.Infof("Un-provisioning topic %s from bus backed by Kafka", topicName)

	err := b.kafkaClusterAdmin.DeleteTopic(topicName)
	if err == sarama.ErrUnknownTopicOrPartition {
		return nil
	} else if err != nil {
		b.logger.Errorf("Error deleting topic %s: %v", topicName, err)
	} else {
		b.logger.Infof("Successfully deleted topic %s", topicName)
	}

	return err
}

func toKafkaMessage(channel buses.ChannelReference, message *buses.Message) *sarama.ProducerMessage {
	kafkaMessage := sarama.ProducerMessage{
		Topic: topicName(channel),
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

func fromKafkaMessage(kafkaMessage *sarama.ConsumerMessage) *buses.Message {
	headers := make(map[string]string)
	for _, header := range kafkaMessage.Headers {
		headers[string(header.Key)] = string(header.Value)
	}
	message := buses.Message{
		Headers: headers,
		Payload: kafkaMessage.Value,
	}
	return &message
}

func topicName(channel buses.ChannelReference) string {
	return fmt.Sprintf("%s.%s", channel.Namespace, channel.Name)
}

func resolveInitialOffset(parameters buses.ResolvedParameters) (int64, error) {
	switch parameters[initialOffset] {
	case oldest:
		return sarama.OffsetOldest, nil
	case newest:
		return sarama.OffsetNewest, nil
	default:
		return 0, fmt.Errorf("unsupported initialOffset value. Must be one of %s or %s", oldest, newest)
	}
}
