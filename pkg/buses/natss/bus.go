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
	"go.uber.org/zap"
	"time"

	"github.com/knative/eventing/pkg/buses"
	stanutil "github.com/knative/eventing/pkg/buses/natss/stanutil"
	stan "github.com/nats-io/go-nats-streaming"
)

// BusType is the type of the stub bus
const (
	BusType  = "natss"
	NatssUrl = "nats://nats-streaming.knative-eventing.svc.cluster.local:4222"
)

type NatssBus struct {
	natssUrl    string
	subscribers map[string]*stan.Subscription

	ref              buses.BusReference
	dispatcher       buses.BusDispatcher
	dispEventHandler buses.EventHandlerFuncs
	provisioner      buses.BusProvisioner
	provEventHandler buses.EventHandlerFuncs

	logger *zap.SugaredLogger
}

type SetupNatssBus func(*NatssBus) error

var (
	natsConn *stan.Conn
)

func NewNatssBusProvisioner(ref buses.BusReference, setup SetupNatssBus) (*NatssBus, error) {
	bus := &NatssBus{
		ref: ref,
	}
	bus.provEventHandler = buses.EventHandlerFuncs{
		ProvisionFunc: func(channel buses.ChannelReference, parameters buses.ResolvedParameters) error {
			bus.logger.Infof("Provision channel %q", channel.Name)
			bus.logger.Infof("channel=%+v; parameters=%+v", channel, parameters)
			return nil
		},
		UnprovisionFunc: func(channel buses.ChannelReference) error {
			bus.logger.Infof("Unprovision channel %q", channel.Name)
			bus.logger.Infof("channel=%+v", channel)
			return nil
		},
	}
	setup(bus)
	return bus, nil
}

func NewNatssBusDispatcher(ref buses.BusReference, setup SetupNatssBus) (*NatssBus, error) {
	bus := &NatssBus{
		ref:         ref,
		subscribers: make(map[string]*stan.Subscription),
	}
	bus.dispEventHandler = buses.EventHandlerFuncs{
		ReceiveMessageFunc: func(channel buses.ChannelReference, message *buses.Message) error {
			bus.logger.Infof("Recieved message from %q channel", channel.String())
			if err := stanutil.Publish(natsConn, channel.Name, &message.Payload, bus.logger); err != nil {
				bus.logger.Errorf("Error during publish: %+v", err)
				return err
			}
			bus.logger.Infof("Published [%s] : '%s'", channel.String(), message)
			return nil
		},
		SubscribeFunc:   bus.subscribe,
		UnsubscribeFunc: bus.unsubscribe,
	}
	setup(bus)
	return bus, nil
}

func (b *NatssBus) Run(threadness int, stopCh <-chan struct{}, clientId string) {
	b.logger.Infof("try to connect to NATSS from %q", clientId)
	var err error
	for i := 0; i < 60; i++ {
		if natsConn, err = stanutil.Connect("knative-nats-streaming", clientId, b.natssUrl, b.logger); err != nil {
			b.logger.Errorf(" Create new connection failed: %+v", err)
			time.Sleep(1 * time.Second)
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

func SetNewBusProvisioner(opts *buses.BusOpts) SetupNatssBus {
	return func(b *NatssBus) error {
		b.natssUrl = NatssUrl
		b.provisioner = buses.NewBusProvisioner(b.ref, b.provEventHandler, opts)
		b.logger = opts.Logger
		return nil
	}
}

func SetNewBusDispatcher(opts *buses.BusOpts) SetupNatssBus {
	return func(b *NatssBus) error {
		b.natssUrl = NatssUrl
		b.dispatcher = buses.NewBusDispatcher(b.ref, b.dispEventHandler, opts)
		b.logger = opts.Logger
		return nil
	}
}

func (bus *NatssBus) subscribe(channel buses.ChannelReference, subscription buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
	bus.logger.Infof("Subscribe %q to %q channel", subscription.Name, channel.Name)

	mcb := func(msg *stan.Msg) {
		bus.logger.Infof("NATSS message received: %+v", msg)
		message := buses.Message{
			Headers: map[string]string{},
			Payload: []byte(msg.Data),
		}
		if err := bus.dispatcher.DispatchMessage(subscription, &message); err != nil {
			bus.logger.Warnf("Failed to dispatch message: %v", err)
			return
		}
		if err := msg.Ack(); err != nil {
			bus.logger.Warnf("Failed to acknowledge message: %v", err)
		}
	}
	// subscribe to a NATSS subject
	if natsStreamingSub, err := (*natsConn).Subscribe(channel.Name, mcb, stan.DurableName(subscription.Name), stan.SetManualAckMode(), stan.AckWait(1*time.Minute)); err != nil {
		bus.logger.Errorf(" Create new NATSS Subscription failed: %+v", err)
		return err
	} else {
		bus.logger.Infof("NATSS Subscription created: %+v", natsStreamingSub)
		bus.subscribers[subscription.Name] = &natsStreamingSub
	}
	return nil
}

func (bus *NatssBus) unsubscribe(channel buses.ChannelReference, subscription buses.SubscriptionReference) error {
	bus.logger.Infof("Unsubscribe %q from %q channel", subscription.Name, channel.Name)

	// unsubscribe from a NATSS subject
	if natsStreamingSub, ok := bus.subscribers[subscription.Name]; ok {
		if err := (*natsStreamingSub).Unsubscribe(); err != nil {
			bus.logger.Errorf(" Unsubscribe() failed: %+v", err)
			return err
		}
		delete(bus.subscribers, subscription.Name)
	}
	return nil
}
