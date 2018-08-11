/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/pkg/signals"
)

const (
	InitialOffset = "initialOffset"
	Newest        = "Newest"
	Oldest        = "Oldest"
)

type dispatcher struct {
	monitor           *buses.Monitor
	messageReceiver   *buses.MessageReceiver
	messageDispatcher *buses.MessageDispatcher
	brokers           []string
	consumers         map[subscriptionKey]*cluster.Consumer
	busName           string
	client            *http.Client
	producer          sarama.AsyncProducer
	informerFactory   informers.SharedInformerFactory
	namespace         string
}

type subscriptionKey struct {
	name      string
	namespace string
}

func main() {

	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL := flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.Parse()

	name := os.Getenv("BUS_NAME")
	namespace := os.Getenv("BUS_NAMESPACE")

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 {
		log.Fatalf("Environment variable KAFKA_BROKERS not set")
	}

	conf := sarama.NewConfig()
	conf.Version = sarama.V1_1_0_0
	conf.ClientID = name + "-dispatcher"
	kafka_client, err := sarama.NewClient(brokers, conf)
	if err != nil {
		glog.Fatalf("Error building kafka client: %v", err)
	}

	dispatcher, err := NewKafkaDispatcher(name, namespace, *masterURL, *kubeconfig, brokers, kafka_client)
	if err != nil {
		glog.Fatalf("Error building kafka provisioner: %v", err)
	}

	stopCh := signals.SetupSignalHandler()
	dispatcher.Start(stopCh)

	<-stopCh
	glog.Flush()
}

func NewKafkaDispatcher(name string, namespace string, masterURL string, kubeConfig string, brokers []string, client sarama.Client) (*dispatcher, error) {

	asyncProducer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case e := <-asyncProducer.Errors():
				glog.Warningf("Got %v", e)
			case s := <-asyncProducer.Successes():
				glog.Infof("Sent %v", s)
			}
		}
	}()

	d := dispatcher{
		busName:   name,
		namespace: namespace,
		client:    &http.Client{},
		brokers:   brokers,
		consumers: make(map[subscriptionKey]*cluster.Consumer),
		producer:  asyncProducer,
	}
	component := fmt.Sprintf("%s-%s", name, buses.Dispatcher)
	monitor := buses.NewMonitor(component, masterURL, kubeConfig, buses.MonitorEventHandlerFuncs{
		SubscribeFunc:   d.subscribe,
		UnsubscribeFunc: d.unsubscribe,
	})
	d.monitor = monitor
	d.messageDispatcher = buses.NewMessageDispatcher()
	d.messageReceiver = buses.NewMessageReceiver(d.handleEvent)

	return &d, nil
}

func (d *dispatcher) Start(stopCh <-chan struct{}) {
	go d.monitor.Run(d.namespace, d.busName, 2, stopCh)
	go d.messageReceiver.Run(stopCh)
}

func (d *dispatcher) subscribe(subscription *channelsv1alpha1.Subscription, parameters buses.ResolvedParameters) error {
	glog.Infof("Subscribing %s/%s: %s -> %s  (%v)", subscription.Namespace,
		subscription.Name, subscription.Spec.Channel, subscription.Spec.Subscriber, parameters)
	channel := &buses.ChannelReference{
		Name:      subscription.Spec.Channel,
		Namespace: subscription.Namespace,
	}
	topicName := topicName(channel)

	initialOffset, err := initialOffset(parameters)
	if err != nil {
		return err
	}

	group := fmt.Sprintf("%s.%s.%s", d.busName, subscription.Namespace, subscription.Name)
	consumerConfig := cluster.NewConfig()
	consumerConfig.Version = sarama.V1_1_0_0
	consumerConfig.Consumer.Offsets.Initial = initialOffset
	consumer, err := cluster.NewConsumer(d.brokers, group, []string{topicName}, consumerConfig)
	if err != nil {
		return err
	}

	d.consumers[subscriptionKeyFor(subscription)] = consumer

	go func() {
		for {
			msg, more := <-consumer.Messages()
			if more {
				glog.Infof("Dispatching a message for subscription %s/%s: %s -> %s", subscription.Namespace,
					subscription.Name, subscription.Spec.Channel, subscription.Spec.Subscriber)
				message := fromKafkaMessage(msg)
				err := d.messageDispatcher.DispatchMessage(subscription.Spec.Subscriber, subscription.Namespace, message)
				if err != nil {
					glog.Warningf("Got error trying to dispatch message: %v", err)
				}
				// TODO: handle errors with pluggable strategy
				consumer.MarkOffset(msg, "") // Mark message as processed
			} else {
				break
			}
		}
		glog.Infof("Consumer for subscription %s/%s stopped", subscription.Namespace, subscription.Name)
	}()

	return nil
}

func initialOffset(parameters buses.ResolvedParameters) (int64, error) {
	sInitial := parameters[InitialOffset]
	if sInitial == Oldest {
		return sarama.OffsetOldest, nil
	} else if sInitial == Newest {
		return sarama.OffsetNewest, nil
	} else {
		return 0, fmt.Errorf("unsupported initialOffset value. Must be one of %s or %s", Oldest, Newest)
	}
}

func (d *dispatcher) unsubscribe(subscription *channelsv1alpha1.Subscription) error {
	glog.Infof("Un-Subscribing %s/%s: %s -> %s", subscription.Namespace,
		subscription.Name, subscription.Spec.Channel, subscription.Spec.Subscriber)
	key := subscriptionKeyFor(subscription)
	if consumer, ok := d.consumers[key]; ok {
		delete(d.consumers, key)
		return consumer.Close()
	}
	return nil
}

func subscriptionKeyFor(subscription *channelsv1alpha1.Subscription) subscriptionKey {
	return subscriptionKey{name: subscription.Name, namespace: subscription.Namespace}
}

func topicName(channel *buses.ChannelReference) string {
	return fmt.Sprintf("%s.%s", channel.Namespace, channel.Name)
}

func (d *dispatcher) handleEvent(channel *buses.ChannelReference, message *buses.Message) error {
	if c := d.monitor.Channel(channel.Name, channel.Namespace); c == nil {
		return buses.ErrUnknownChannel
	}

	d.producer.Input() <- toKafkaMessage(channel, message)

	return nil
}

func toKafkaMessage(channel *buses.ChannelReference, message *buses.Message) *sarama.ProducerMessage {
	kafkaMessage := sarama.ProducerMessage{
		Topic: topicName(channel),
		Value: sarama.ByteEncoder(message.Payload),
	}
	for h, v := range message.Headers {
		kafkaMessage.Headers = append(kafkaMessage.Headers, sarama.RecordHeader{[]byte(h), []byte(v)})
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
