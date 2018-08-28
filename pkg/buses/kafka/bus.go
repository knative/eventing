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

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/buses"
)

const (
	initialOffset = "initialOffset"
	numPartitions = "NumPartitions"
	newest        = "Newest"
	oldest        = "Oldest"
)

type KafkaBus struct {
	busRef             buses.BusReference
	dispatcher         buses.BusDispatcher
	provisioner        buses.BusProvisioner
	kafkaBrokers       []string
	kafkaClusterAdmin  sarama.ClusterAdmin
	kafkaAsyncProducer sarama.AsyncProducer
	kafkaConsumers     map[buses.SubscriptionReference]*cluster.Consumer
}

func NewKafkaBusDispatcher(busRef buses.BusReference, brokers []string, opts *buses.BusOpts) (*KafkaBus, error) {
	bus := &KafkaBus{
		busRef:       busRef,
		kafkaBrokers: brokers,
	}
	eventHandlers := buses.EventHandlerFuncs{
		SubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			return bus.subscribe(channelRef, subscriptionRef, parameters)
		},
		UnsubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference) error {
			return bus.unsubscribe(channelRef, subscriptionRef)
		},
		ReceiveMessageFunc: func(channelRef buses.ChannelReference, message *buses.Message) error {
			bus.kafkaAsyncProducer.Input() <- toKafkaMessage(channelRef, message)
			return nil
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(busRef, eventHandlers, opts)
	bus.kafkaConsumers = make(map[buses.SubscriptionReference]*cluster.Consumer)

	conf := sarama.NewConfig()
	conf.Version = sarama.V1_1_0_0
	conf.ClientID = busRef.Name + "-dispatcher"
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

func NewKafkaBusProvisioner(busRef buses.BusReference, brokers []string, opts *buses.BusOpts) (*KafkaBus, error) {
	bus := &KafkaBus{
		busRef:       busRef,
		kafkaBrokers: brokers,
	}
	eventHandlers := buses.EventHandlerFuncs{
		ProvisionFunc: func(channelRef buses.ChannelReference, parameters buses.ResolvedParameters) error {
			return bus.provision(channelRef, parameters)
		},
		UnprovisionFunc: func(channelRef buses.ChannelReference) error {
			return bus.unprovision(channelRef)
		},
	}
	bus.provisioner = buses.NewBusProvisioner(busRef, eventHandlers, opts)

	conf := sarama.NewConfig()
	conf.Version = sarama.V1_1_0_0
	conf.ClientID = busRef.Name + "-provisioner"

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
					glog.Warningf("Got %v", e)
				case s := <-b.kafkaAsyncProducer.Successes():
					glog.Infof("Sent %v", s)
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

func (b *KafkaBus) subscribe(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
	if _, ok := b.kafkaConsumers[subscriptionRef]; ok {
		// subscribe can be called multiple times for the same subscription,
		//unsubscribe before we resubscribe
		err := b.unsubscribe(channelRef, subscriptionRef)
		if err != nil {
			return err
		}
	}

	glog.Infof("Subscribing %q to %q (%v)", subscriptionRef.String(), channelRef.String(), parameters)

	topicName := topicName(channelRef)

	initialOffset, err := resolveInitialOffset(parameters)
	if err != nil {
		return err
	}

	group := fmt.Sprintf("%s.%s.%s", b.busRef.Name, subscriptionRef.Namespace, subscriptionRef.Name)
	consumerConfig := cluster.NewConfig()
	consumerConfig.Version = sarama.V1_1_0_0
	consumerConfig.Consumer.Offsets.Initial = initialOffset
	consumer, err := cluster.NewConsumer(b.kafkaBrokers, group, []string{topicName}, consumerConfig)
	if err != nil {
		return err
	}

	b.kafkaConsumers[subscriptionRef] = consumer

	go func() {
		for {
			msg, more := <-consumer.Messages()
			if more {
				glog.Infof("Dispatching a message for subscription %q", subscriptionRef.String())
				message := fromKafkaMessage(msg)
				err := b.dispatcher.DispatchMessage(subscriptionRef, message)
				if err != nil {
					glog.Warningf("Got error trying to dispatch message: %v", err)
				}
				// TODO: handle errors with pluggable strategy
				consumer.MarkOffset(msg, "") // Mark message as processed
			} else {
				break
			}
		}
		glog.Infof("Consumer for subscription %q stopped", subscriptionRef.String())
	}()

	return nil
}

func (b *KafkaBus) unsubscribe(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference) error {
	glog.Infof("Un-Subscribing %q from %q", subscriptionRef.String(), channelRef.String())
	if consumer, ok := b.kafkaConsumers[subscriptionRef]; ok {
		delete(b.kafkaConsumers, subscriptionRef)
		return consumer.Close()
	}
	return nil
}

func (b *KafkaBus) provision(channelRef buses.ChannelReference, parameters buses.ResolvedParameters) error {
	topicName := topicName(channelRef)
	glog.Infof("Provisioning topic %s on bus backed by Kafka", topicName)

	partitions := 1
	if p, ok := parameters[numPartitions]; ok {
		var err error
		partitions, err = strconv.Atoi(p)
		if err != nil {
			glog.Warningf("Could not parse partition count for %q: %s", channelRef.String(), p)
		}
	}

	err := b.kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
		ReplicationFactor: 1,
		NumPartitions:     int32(partitions),
	}, false)
	if err == sarama.ErrTopicAlreadyExists {
		return nil
	} else if err != nil {
		glog.Errorf("Error creating topic %s: %v", topicName, err)
	} else {
		glog.Infof("Successfully created topic %s", topicName)
	}
	return err
}

func (b *KafkaBus) unprovision(channelRef buses.ChannelReference) error {
	topicName := topicName(channelRef)
	glog.Infof("Un-provisioning topic %s from bus backed by Kafka", topicName)

	err := b.kafkaClusterAdmin.DeleteTopic(topicName)
	if err == sarama.ErrUnknownTopicOrPartition {
		return nil
	} else if err != nil {
		glog.Errorf("Error deleting topic %s: %v", topicName, err)
	} else {
		glog.Infof("Successfully deleted topic %s", topicName)
	}

	return err
}

func toKafkaMessage(channelRef buses.ChannelReference, message *buses.Message) *sarama.ProducerMessage {
	kafkaMessage := sarama.ProducerMessage{
		Topic: topicName(channelRef),
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
