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

package natss

import (
	"time"
	"go.uber.org/zap"

	stan "github.com/nats-io/go-nats-streaming"
	stanutil "github.com/knative/eventing/pkg/buses/natss/stanutil"
	"github.com/knative/eventing/pkg/buses"
)

// BusType is the type of the stub bus
const BusType = "natss"

type NatssBus struct {
	natsConn *stan.Conn
	subscribers map[string]*stan.Subscription

	ref         buses.BusReference
	dispatcher  buses.BusDispatcher
	provisioner buses.BusProvisioner

	logger *zap.SugaredLogger
}

var (
	natsConn *stan.Conn
)

func NewNatssBusProvisioner(ref buses.BusReference, opts *buses.BusOpts) (*NatssBus, error) {
	bus := &NatssBus{
		ref: ref,
		subscribers: make(map[string]*stan.Subscription),
	}
	eventHandlers := buses.EventHandlerFuncs{
		ProvisionFunc: func(channel buses.ChannelReference, parameters buses.ResolvedParameters) error {
			bus.logger.Infof("Provision channel %q\n", channel.Name)
			bus.logger.Info("channel=%+v; parameters=%+v", channel, parameters)
			// TODO create a NATSS subject
			return nil
		},
		UnprovisionFunc: func(channel buses.ChannelReference) error {
			bus.logger.Infof("Unprovision channel %q\n", channel.Name)
			bus.logger.Info("channel=%+v", channel)
			// TODO delete a NATSS subject
			return nil
		},
		SubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			bus.logger.Infof("Subscribe %q to %q channel\n", subscription.Name, channel.Name)
			bus.logger.Info("subscription= %+v; parameters=%+v", subscription, parameters)

			mcb := func(msg *stan.Msg) {
				bus.logger.Info("NATSS message received: %+v", msg)
				message := buses.Message{
					Headers: map[string]string{},
					Payload: []byte(msg.String()),
				}
				if err := bus.dispatcher.DispatchMessage(subscription, &message); err != nil {
					bus.logger.Warnf("Failed to dispatch message: %v", err)
				}
			}
			// subscribe to a NATSS subject
			if natsStreamingSub, err := (*natsConn).Subscribe(channel.Name, mcb /*nmh.msgh*/, stan.DurableName(subscription.Name)); err != nil {
				bus.logger.Errorf(" Create new NATSS Subscription failed: %+v", err)
			} else {
				bus.logger.Info("NATSS Subscription created: %+v", natsStreamingSub)
				bus.subscribers[subscription.Name] = &natsStreamingSub
			}
			return nil
		},
		UnsubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference) error {
			bus.logger.Infof("Unsubscribe %q from %q channel\n", subscription.Name, channel.Name)
			bus.logger.Info("subscription= %+v", subscription)

			// unsubscribe from a NATSS subject
			if natsStreamingSub, ok := bus.subscribers[subscription.Name]; ok {
				if err := (*natsStreamingSub).Unsubscribe(); err  != nil {
					bus.logger.Errorf(" Unsubscribe() failed: %+v", err)
				} else {
					bus.logger.Errorf(" Unsubscribe() successful.")
				}
			}
			return nil
		},
	}

	bus.provisioner = buses.NewBusProvisioner(ref, eventHandlers, opts)
	bus.dispatcher = buses.NewBusDispatcher(ref, buses.EventHandlerFuncs{}, opts)
	bus.logger = opts.Logger

	return bus, nil
}

func NewNatssBusDispatcher(ref buses.BusReference, opts *buses.BusOpts) (*NatssBus, error) {
	bus := &NatssBus{}
	handlerFuncs := buses.EventHandlerFuncs{
		ReceiveMessageFunc: func(channel buses.ChannelReference, message *buses.Message) error {
			bus.logger.Infof("Recieved message for %q channel", channel.String())
			if err := stanutil.Publish(natsConn, channel.Name, &message.Payload); err != nil {
				bus.logger.Errorf("Error during publish: %+v", err)
			} else {
				bus.logger.Infof("Published [%s] : '%s'", channel.String(), message)
			}
			return nil
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(ref, handlerFuncs, opts)
	bus.logger = opts.Logger

	return bus, nil
}

func (b *NatssBus) Run(threadness int, stopCh <-chan struct{}, clientId string) {
	b.logger.Infof("try to connect to NATSS from %s", clientId)
	var err error
	for i:=0; i<60; i++ {
		if natsConn, err = stanutil.Connect("knative-nats-streaming", clientId, "nats://nats-streaming-headless.default.svc.cluster.local:4222"); err != nil {
			b.logger.Errorf(" Create new connection failed: %+v", err)
			time.Sleep(time.Duration(1)*time.Second)
		} else {
			break
		}
	}
	if err != nil {
		b.logger.Errorf(" Create new connection failed: %+v", err)
		return
	}
	b.logger.Info("connection to NATSS established, natsConn=%+v", natsConn)

	if b.dispatcher != nil {
		b.dispatcher.Run(threadness, stopCh)
	}
	if b.provisioner != nil {
		b.provisioner.Run(threadness, stopCh)
	}
}

