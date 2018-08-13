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
	"github.com/knative/pkg/signals"
)

const (
	threadsPerMonitor = 1
)

var (
	masterURL  string
	kubeconfig string
)

// StubBus is able to broadcast messages to multiple subscribers, but does not
// have any delivery guarantees.
//
// The stub bus is commonly used in development and testing, but is often not
// suitable for production environments.
type StubBus struct {
	ref        *buses.BusReference
	monitor    *buses.Monitor
	receiver   *buses.MessageReceiver
	dispatcher *buses.MessageDispatcher
}

// NewStubBus creates a stub bus.
func NewStubBus(ref *buses.BusReference, monitor *buses.Monitor) *StubBus {
	bus := &StubBus{
		ref:     ref,
		monitor: monitor,
	}
	bus.dispatcher = buses.NewMessageDispatcher()
	bus.receiver = buses.NewMessageReceiver(bus.receiveMessage)
	return bus
}

// Run starts the bus's monitor and receiver. This function will block until
// the stop channel receives a message.
func (b *StubBus) Run(stopCh <-chan struct{}) {
	go func() {
		if err := b.monitor.Run(b.ref.Namespace, b.ref.Name, 1, stopCh); err != nil {
			glog.Fatalf("Error running monitor: %s", err.Error())
		}
	}()
	b.monitor.WaitForCacheSync(stopCh)
	b.receiver.Run(stopCh)
}

// receiveMessage receives new messages for the bus from the message receiver,
// looks up active subscriptions for the channel and dispatches the message to
// each subscriber.
func (b *StubBus) receiveMessage(channel *buses.ChannelReference, message *buses.Message) error {
	subscriptions := b.monitor.Subscriptions(channel.Name, channel.Namespace)
	if subscriptions == nil {
		return buses.ErrUnknownChannel
	}
	for _, subscription := range *subscriptions {
		go b.dispatchMessage(subscription, channel, message)
	}
	return nil
}

// dispatchMessage dispatches messages for the bus to a channel's subscriber.
func (b *StubBus) dispatchMessage(subscription channelsv1alpha1.SubscriptionSpec, channel *buses.ChannelReference, message *buses.Message) {
	subscriber := subscription.Subscriber
	glog.Infof("Sending to %q for %q", subscriber, channel)
	b.dispatcher.DispatchMessage(subscriber, channel.Namespace, message)
}

func main() {
	defer glog.Flush()

	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	busReference := &buses.BusReference{
		Namespace: os.Getenv("BUS_NAMESPACE"),
		Name:      os.Getenv("BUS_NAME"),
	}
	component := fmt.Sprintf("%s-%s", busReference.Name, buses.Dispatcher)

	monitor := buses.NewMonitor(component, masterURL, kubeconfig, buses.MonitorEventHandlerFuncs{
		ProvisionFunc: func(channel *channelsv1alpha1.Channel, parameters buses.ResolvedParameters) error {
			glog.Infof("Provision channel %q\n", channel.Name)
			return nil
		},
		UnprovisionFunc: func(channel *channelsv1alpha1.Channel) error {
			glog.Infof("Unprovision channel %q\n", channel.Name)
			return nil
		},
		SubscribeFunc: func(subscription *channelsv1alpha1.Subscription, parameters buses.ResolvedParameters) error {
			glog.Infof("Subscribe %q to %q channel\n", subscription.Spec.Subscriber, subscription.Spec.Channel)
			return nil
		},
		UnsubscribeFunc: func(subscription *channelsv1alpha1.Subscription) error {
			glog.Infof("Unsubscribe %q from %q channel\n", subscription.Spec.Subscriber, subscription.Spec.Channel)
			return nil
		},
	})
	bus := NewStubBus(busReference, monitor)
	bus.Run(stopCh)
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
