/*
 * Copyright 2018 the original author or authors.
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

package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/signals"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	masterURL  string
	kubeconfig string
)

type StubBus struct {
	name           string
	monitor        *buses.Monitor
	client         *http.Client
	forwardHeaders []string
}

func (b *StubBus) handleEvent(res http.ResponseWriter, req *http.Request) {
	host := req.Host
	glog.Infof("Received request for %s\n", host)
	channel, namespace := b.splitChannelName(host)
	subscriptions := b.monitor.Subscriptions(channel, namespace)
	if subscriptions == nil {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusAccepted)

	if len(*subscriptions) == 0 {
		glog.Warningf("No subscribers for channel %q\n", channel)
	}

	safeHeaders := b.safeHeaders(req.Header)
	safeHeaders.Set("x-bus", b.name)
	safeHeaders.Set("x-channel", channel)
	for _, subscription := range *subscriptions {
		subscriber := subscription.Subscriber
		glog.Infof("Sending to %q for %q\n", subscriber, channel)
		go b.dispatchEvent(subscriber, body, safeHeaders)
	}
}

func (b *StubBus) dispatchEvent(subscriber string, body []byte, headers http.Header) {
	url := url.URL{
		Scheme: "http",
		Host:   subscriber,
		Path:   "/",
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(body))
	if err != nil {
		glog.Errorf("Unable to create subscriber request %v", err)
	}
	req.Header = headers
	_, err = b.client.Do(req)
	if err != nil {
		glog.Errorf("Unable to complete subscriber request %v", err)
	}
}

func (b *StubBus) splitChannelName(host string) (string, string) {
	chunks := strings.Split(host, ".")
	channel := chunks[0]
	namespace := chunks[1]
	return channel, namespace
}

func (b *StubBus) safeHeaders(raw http.Header) http.Header {
	safe := http.Header{}
	for _, header := range b.forwardHeaders {
		if value := raw.Get(header); value != "" {
			safe.Set(header, value)
		}
	}
	return safe
}

func NewStubBus(name string, monitor *buses.Monitor) *StubBus {
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

	bus := StubBus{
		name:           name,
		monitor:        monitor,
		client:         &http.Client{},
		forwardHeaders: forwardHeaders,
	}

	return &bus
}

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building clientset: %s", err.Error())
	}

	name := os.Getenv("BUS_NAME")

	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	monitor := buses.NewMonitor(name, informerFactory, buses.MonitorEventHandlerFuncs{
		ProvisionFunc: func(channel channelsv1alpha1.Channel) {
			glog.Infof("Provision channel %q\n", channel.Name)
		},
		UnprovisionFunc: func(channel channelsv1alpha1.Channel) {
			glog.Infof("Unprovision channel %q\n", channel.Name)
		},
		SubscribeFunc: func(subscription channelsv1alpha1.Subscription) {
			glog.Infof("Subscribe %q to %q channel\n", subscription.Spec.Subscriber, subscription.Spec.Channel)
		},
		UnsubscribeFunc: func(subscription channelsv1alpha1.Subscription) {
			glog.Infof("Unubscribe %q from %q channel\n", subscription.Spec.Subscriber, subscription.Spec.Channel)
		},
	})
	bus := NewStubBus(name, monitor)

	go informerFactory.Start(stopCh)

	http.HandleFunc("/", bus.handleEvent)
	glog.Fatal(http.ListenAndServe(":8080", nil))

	glog.Flush()
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
