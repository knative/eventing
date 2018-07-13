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
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/signals"
)

const (
	InitialOffset = "initialOffset"
	Newest        = "Newest"
	Oldest        = "Oldest"
)

type dispatcher struct {
	monitor         *buses.Monitor
	brokers         []string
	consumers       map[subscriptionKey]*cluster.Consumer
	busName         string
	client          *http.Client
	producer        sarama.AsyncProducer
	informerFactory informers.SharedInformerFactory
	namespace       string
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

	return &d, nil
}

func (d *dispatcher) Start(stopCh <-chan struct{}) {
	go d.monitor.Run(d.namespace, d.busName, 2, stopCh)
	d.startHttpServer(stopCh)
}

func (d *dispatcher) startHttpServer(stopCh <-chan struct{}) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.handleEvent)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%v", 8080),
		Handler: mux,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			glog.Fatalf("Error running http server: %v", err)
		} else {
			glog.Infof("Http server shut down")
		}
	}()
	go func() {
		<-stopCh
		timeout, ctx := context.WithTimeout(context.Background(), 1*time.Second)
		defer ctx()
		if err := httpServer.Shutdown(timeout); err != nil {
			glog.Fatalf("Failure to shut down http server gracefully: %v", err)
		}
	}()
}

func (d *dispatcher) subscribe(subscription *channelsv1alpha1.Subscription, parameters buses.ResolvedParameters) error {
	glog.Infof("Subscribing %s/%s: %s -> %s  (%v)", subscription.Namespace,
		subscription.Name, subscription.Spec.Channel, subscription.Spec.Subscriber, parameters)
	topicName := topicNameFromMeta(subscription.Spec.Channel, subscription.Namespace)

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
				hs := kafka2HttpHeaders(msg)
				svc := fmt.Sprintf("%s.%s", subscription.Spec.Subscriber, subscription.Namespace)
				err := d.dispatchEvent(svc, msg.Value, hs)
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

func kafka2HttpHeaders(message *sarama.ConsumerMessage) http.Header {
	result := make(http.Header)
	for _, h := range message.Headers {
		result[string(h.Key)] = []string{string(h.Value)}
	}
	return result
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

func topicNameFromMeta(channel string, namespace string) string {
	return fmt.Sprintf("%s.%s", namespace, channel)
}

func (d *dispatcher) dispatchEvent(host string, body []byte, headers http.Header) error {
	url := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   "/",
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = headers
	_, err = d.client.Do(req)
	return err
}

func channelFromHost(host string) (channel string, namespace string) {
	chunks := strings.Split(host, ".")
	channel = chunks[0]
	if len(chunks) > 1 {
		namespace = chunks[1]
	}
	return
}

func (d *dispatcher) handleEvent(res http.ResponseWriter, req *http.Request) {
	host := req.Host
	glog.Infof("Received request for %s\n", host)
	channel, namespace := channelFromHost(host)
	c := d.monitor.Channel(channel, namespace)
	if c == nil {
		res.WriteHeader(http.StatusNotFound)
	}

	msg, err := http2KafkaMessage(channel, namespace, req)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	d.producer.Input() <- msg

	res.WriteHeader(http.StatusAccepted)
}

func http2KafkaMessage(channel string, namespace string, req *http.Request) (*sarama.ProducerMessage, error) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	message := sarama.ProducerMessage{
		Topic: topicNameFromMeta(channel, namespace),
		Value: sarama.ByteEncoder(body),
	}
	for h, v := range req.Header {
		message.Headers = append(message.Headers, sarama.RecordHeader{[]byte(h), []byte(v[0])})
	}
	return &message, nil
}
