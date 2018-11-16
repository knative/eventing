/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dispatcher

import (
	"sync"
	"time"

	"github.com/knative/eventing/pkg/buses"
	"github.com/knative/eventing/pkg/provisioners/natss/controller/clusterchannelprovisioner"
	"github.com/knative/eventing/pkg/provisioners/natss/stanutil"
	"github.com/nats-io/go-nats-streaming"
	"go.uber.org/zap"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

const (
	clientId = "knative-natss-dispatcher"
)

// SubscriptionsSupervisor manages the state of NATS Streaming subscriptions
type SubscriptionsSupervisor struct {
	logger *zap.Logger

	receiver   *buses.MessageReceiver
	dispatcher *buses.MessageDispatcher

	natssConn *stan.Conn

	subscriptionsMux sync.Mutex
	subscriptions    map[buses.ChannelReference]map[subscriptionReference]*stan.Subscription
}

func NewDispatcher(logger *zap.Logger) (*SubscriptionsSupervisor, error) {
	d := &SubscriptionsSupervisor{
		logger:        logger,
		dispatcher:    buses.NewMessageDispatcher(logger.Sugar()),
		subscriptions: make(map[buses.ChannelReference]map[subscriptionReference]*stan.Subscription),
	}
	nConn, err := stanutil.Connect(clusterchannelprovisioner.ClusterId, clientId, clusterchannelprovisioner.NatssUrl, d.logger.Sugar())
	if err != nil {
		logger.Error("Connect() failed: ", zap.Error(err))
	}
	d.natssConn = nConn
	d.receiver = buses.NewMessageReceiver(createReceiverFunction(d, logger.Sugar()), logger.Sugar())

	return d, nil
}

func createReceiverFunction(s *SubscriptionsSupervisor, logger *zap.SugaredLogger) func(buses.ChannelReference, *buses.Message) error {
	return func(channel buses.ChannelReference, m *buses.Message) error {
		logger.Infof("Received message from %q channel", channel.String())
		// publish to Natss
		ch := channel.Name + "." + channel.Namespace
		if err := stanutil.Publish(s.natssConn, ch, &m.Payload, logger); err != nil {
			logger.Errorf("Error during publish: %v", err)
			return err
		}
		logger.Infof("Published [%s] : '%s'", channel.String(), m.Headers)
		return nil
	}
}

func (s *SubscriptionsSupervisor) Start(stopCh <-chan struct{}) error {
	s.receiver.Run(stopCh)
	return nil
}

func (s *SubscriptionsSupervisor) UpdateSubscriptions(channel eventingv1alpha1.Channel) error {
	if channel.Spec.Subscribable == nil {
		s.logger.Sugar().Infof("Empty subscriptions for channel: %v ; nothing to update ", channel)
		return nil
	}

	subscriptions := channel.Spec.Subscribable.Subscribers
	activeSubs := make(map[subscriptionReference]bool)

	cRef := buses.NewChannelReferenceFromNames(channel.Name, channel.Namespace)

	s.subscriptionsMux.Lock()
	defer s.subscriptionsMux.Unlock()

	chMap, ok := s.subscriptions[cRef]
	if !ok {
		chMap = make(map[subscriptionReference]*stan.Subscription)
		s.subscriptions[cRef] = chMap
	}
	for _, sub := range subscriptions {
		// check if the subscribtion already exist and do nothing in this case
		if _, ok := chMap[newSubscriptionReference(sub)]; ok {
			activeSubs[newSubscriptionReference(sub)] = true
			s.logger.Sugar().Infof("Subscription: %v already active for channel: %v", sub, cRef)
			continue
		}
		// subscribe
		if natssSub, err := s.subscribe(cRef, newSubscriptionReference(sub)); err != nil {
			return err
		} else {
			chMap[newSubscriptionReference(sub)] = natssSub
			activeSubs[newSubscriptionReference(sub)] = true
		}
	}
	// Unsubscribe for deleted subscriptions
	for sub := range chMap {
		if ok := activeSubs[sub]; !ok {
			s.unsubscribe(cRef, sub)
		}
	}
	// delete the channel from s.subscriptions if chMap is empty
	if len(s.subscriptions[cRef]) == 0 {
		delete(s.subscriptions, cRef)
	}
	return nil
}

func (s *SubscriptionsSupervisor) subscribe(channel buses.ChannelReference, subscription subscriptionReference) (*stan.Subscription, error) {
	s.logger.Info("Subscribe to channel:", zap.Any("channel", channel), zap.Any("subscription", subscription))

	mcb := func(msg *stan.Msg) {
		s.logger.Sugar().Infof("NATSS message received from subject: %v; sequence: %v; timestamp: %v", msg.Subject, msg.Sequence, msg.Timestamp)
		message := buses.Message{
			Headers: map[string]string{},
			Payload: []byte(msg.Data),
		}
		if err := s.dispatcher.DispatchMessage(&message, subscription.SubscriberURI, subscription.ReplyURI, buses.DispatchDefaults{}); err != nil {
			s.logger.Error("Failed to dispatch message: ", zap.Error(err))
			return
		}
		if err := msg.Ack(); err != nil {
			s.logger.Error("Failed to acknowledge message: ", zap.Error(err))
		}
	}
	// subscribe to a NATSS subject
	ch := channel.Name + "." + channel.Namespace
	sub := subscription.Name + "." + subscription.Namespace
	if natssSub, err := (*s.natssConn).Subscribe(ch, mcb, stan.DurableName(sub), stan.SetManualAckMode(), stan.AckWait(1*time.Minute)); err != nil {
		s.logger.Error(" Create new NATSS Subscription failed: ", zap.Error(err))
		return nil, err
	} else {
		s.logger.Sugar().Infof("NATSS Subscription created: %+v", natssSub)
		return &natssSub, nil
	}
}

func (s *SubscriptionsSupervisor) unsubscribe(channel buses.ChannelReference, subscription subscriptionReference) error {
	s.logger.Info("Unsubscribe from channel:", zap.Any("channel", channel), zap.Any("subscription", subscription))

	if stanSub, ok := s.subscriptions[channel][subscription]; ok {
		// delete from NATSS
		if err := (*stanSub).Unsubscribe(); err != nil {
			s.logger.Error("Unsubscribing NATS Streaming subscription failed: ", zap.Error(err))
		}
		delete(s.subscriptions[channel], subscription)
	}
	return nil
}
