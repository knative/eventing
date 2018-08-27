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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	"github.com/knative/eventing/pkg/buses/gcppubsub"
	"github.com/knative/pkg/signals"
)

const (
	threadsPerMonitor = 1
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	defer glog.Flush()

	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	namespace := os.Getenv("BUS_NAMESPACE")
	name := os.Getenv("BUS_NAME")
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		glog.Fatalf("GOOGLE_CLOUD_PROJECT environment variable must be set.\n")
	}

	component := fmt.Sprintf("%s-%s", name, buses.Dispatcher)

	var bus *gcppubsub.PubSubBus
	monitor := buses.NewMonitor(component, masterURL, kubeconfig, buses.MonitorEventHandlerFuncs{
		SubscribeFunc: func(subscription *channelsv1alpha1.Subscription, parameters buses.ResolvedParameters) error {
			return bus.ReceiveEvents(subscription, parameters)
		},
		UnsubscribeFunc: func(subscription *channelsv1alpha1.Subscription) error {
			return bus.StopReceiveEvents(subscription)
		},
	})
	messageReceiver := buses.NewMessageReceiver(func(channel *buses.ChannelReference, message *buses.Message) error {
		return bus.ReceiveMessage(channel, message)
	})
	messageDispatcher := buses.NewMessageDispatcher()
	bus, err := gcppubsub.NewPubSubBus(name, projectID, monitor, messageReceiver, messageDispatcher)
	if err != nil {
		glog.Fatalf("Failed to create pubsub bus: %v", err)
	}

	go func() {
		if err := monitor.Run(namespace, name, threadsPerMonitor, stopCh); err != nil {
			glog.Fatalf("Error running monitor: %v", err)
		}
	}()
	messageReceiver.Run(stopCh)
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
