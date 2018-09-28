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

package buses

import (
	"fmt"

	"go.uber.org/zap"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
)

type bus struct {
	ref          BusReference
	handlerFuncs EventHandlerFuncs

	reconciler *Reconciler
	cache      *Cache
	dispatcher *MessageDispatcher
	receiver   *MessageReceiver

	logger *zap.SugaredLogger
}

// BusOpts holds configuration options for new buses. These options are not
// required for proper operation of the bus, but are useful to override the
// default behavior and for testing.
type BusOpts struct {
	// Logger to use for the bus and created components
	Logger *zap.SugaredLogger

	// MasterURL is the address of the Kubernetes API server. Overrides any
	// value in kubeconfig. Only required if out-of-cluster.
	MasterURL string
	// KubeConfig is the path to a kubeconfig. Only required if out-of-cluster.
	KubeConfig string

	// Cache to use for this bus. A cache will be created for the bus if not
	// specified.
	Cache *Cache
	// Reconciler to use for this bus. A reconciler wil be created for the bus
	// if not specified.
	Reconciler *Reconciler
	// MessageDispatcher to use for this bus. The message dispatcher is used to
	// send messages from the bus to a subscriber. A message dispatcher will be
	// created for the bus if needed and not specified.
	MessageDispatcher *MessageDispatcher
	// MessageReceiver to use for this bus. The message receiver is used to
	// receive message sent to a channel. A message receiver will be created
	// for the bus if needed and not specified.
	MessageReceiver *MessageReceiver
}

// BusProvisioner provisions channels and subscriptions for a bus on backing
// infrastructure.
type BusProvisioner interface {
	Run(threadiness int, stopCh <-chan struct{})
}

// NewBusProvisioner creates a new provisioner for a specific bus.
// EventHandlerFuncs are used to be notified when a channel or subscription is
// created, updated or removed.
func NewBusProvisioner(ref BusReference, handlerFuncs EventHandlerFuncs, opts *BusOpts) BusProvisioner {
	if opts == nil {
		opts = &BusOpts{}
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop().Sugar()
	}
	if opts.Cache == nil {
		opts.Cache = NewCache()
	}
	if handlerFuncs.logger == nil {
		handlerFuncs.logger = opts.Logger.Named(handlerLoggingComponent)
	}
	if opts.Reconciler == nil {
		opts.Reconciler = NewReconciler(
			ref, Provisioner, opts.MasterURL, opts.KubeConfig, opts.Cache,
			handlerFuncs, opts.Logger.Named(reconcilerLoggingComponent),
		)
	}

	return &bus{
		ref:          ref,
		handlerFuncs: handlerFuncs,

		cache:      opts.Cache,
		reconciler: opts.Reconciler,

		logger: opts.Logger.Named(busLoggingComponent),
	}
}

// BusDispatcher dispatches messages from channels to subscribers via backing
// infrastructure.
type BusDispatcher interface {
	Run(threadiness int, stopCh <-chan struct{})
	DispatchMessage(subscription SubscriptionReference, message *Message) error
}

// NewBusDispatcher creates a new dispatcher for a specific bus.
// EventHandlerFuncs are used to be notified when a subscription is created,
// updated or removed, or a message is received.
func NewBusDispatcher(ref BusReference, handlerFuncs EventHandlerFuncs, opts *BusOpts) BusDispatcher {
	var b *bus

	if opts == nil {
		opts = &BusOpts{}
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop().Sugar()
	}
	if opts.Cache == nil {
		opts.Cache = NewCache()
	}
	if handlerFuncs.logger == nil {
		handlerFuncs.logger = opts.Logger.Named(handlerLoggingComponent)
	}
	if opts.Reconciler == nil {
		opts.Reconciler = NewReconciler(
			ref, Dispatcher, opts.MasterURL, opts.KubeConfig, opts.Cache,
			handlerFuncs, opts.Logger.Named(reconcilerLoggingComponent),
		)
	}
	if opts.MessageDispatcher == nil {
		opts.MessageDispatcher = NewMessageDispatcher(opts.Logger.Named(dispatcherLoggingComponent))
	}
	if opts.MessageReceiver == nil {
		opts.MessageReceiver = NewMessageReceiver(func(channel ChannelReference, message *Message) error {
			return b.receiveMessage(channel, message)
		}, opts.Logger.Named(receiverLoggingComponent))
	}

	b = &bus{
		ref:          ref,
		handlerFuncs: handlerFuncs,

		cache:      opts.Cache,
		reconciler: opts.Reconciler,
		dispatcher: opts.MessageDispatcher,
		receiver:   opts.MessageReceiver,

		logger: opts.Logger,
	}

	return b
}

// Run starts the bus's processing.
func (b bus) Run(threadiness int, stopCh <-chan struct{}) {
	go b.reconciler.Run(threadiness, stopCh)
	b.reconciler.WaitForCacheSync(stopCh)
	if b.receiver != nil {
		go b.receiver.Run(stopCh)
	}

	<-stopCh
}

func (b *bus) receiveMessage(channel ChannelReference, message *Message) error {
	_, err := b.cache.Channel(channel)
	if err != nil {
		return ErrUnknownChannel
	}
	return b.handlerFuncs.onReceiveMessage(channel, message)
}

// DispatchMessage sends a message to a subscriber. This function is only
// avilable for bus dispatchers.
func (b *bus) DispatchMessage(ref SubscriptionReference, message *Message) error {
	subscription, err := b.cache.Subscription(ref)
	if err != nil {
		return fmt.Errorf("unable to dispatch to unknown subscription %q", ref.String())
	}
	return b.dispatchMessage(subscription, message)
}

func (b *bus) dispatchMessage(subscription *channelsv1alpha1.Subscription, message *Message) error {
	subscriber := subscription.Spec.Subscriber
	defaults := DispatchDefaults{
		Namespace: subscription.Namespace,
	}
	return b.dispatcher.DispatchMessage(message, subscriber, subscription.Spec.ReplyTo, defaults)
}
