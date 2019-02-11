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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/knative/eventing/contrib/natss/pkg/controller/clusterchannelprovisioner"
	"github.com/knative/eventing/contrib/natss/pkg/stanutil"
	"github.com/knative/eventing/pkg/provisioners"
	stan "github.com/nats-io/go-nats-streaming"
	"go.uber.org/zap"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

const clientID = "knative-natss-dispatcher"

// SubscriptionsSupervisor manages the state of NATS Streaming subscriptions
type SubscriptionsSupervisor struct {
	logger *zap.Logger

	receiver            *provisioners.MessageReceiver
	dispatcher          *provisioners.MessageDispatcher
	connect             chan struct{}
	natssURL            string
	subscriptionsMux    sync.Mutex
	subscriptions       map[provisioners.ChannelReference]map[subscriptionReference]*stan.Subscription
	natssConn           *stan.Conn
	natssConnInProgress bool
}

// NewDispatcher returns a new SubscriptionsSupervisor.
func NewDispatcher(natssUrl string, logger *zap.Logger) (*SubscriptionsSupervisor, error) {
	d := &SubscriptionsSupervisor{
		logger:        logger,
		dispatcher:    provisioners.NewMessageDispatcher(logger.Sugar()),
		connect:       make(chan struct{}),
		natssURL:      natssUrl,
		subscriptions: make(map[provisioners.ChannelReference]map[subscriptionReference]*stan.Subscription),
	}
	d.receiver = provisioners.NewMessageReceiver(createReceiverFunction(d, logger.Sugar()), logger.Sugar())

	return d, nil
}

func createReceiverFunction(s *SubscriptionsSupervisor, logger *zap.SugaredLogger) func(provisioners.ChannelReference, *provisioners.Message) error {
	return func(channel provisioners.ChannelReference, m *provisioners.Message) error {
		logger.Infof("Received message from %q channel", channel.String())
		// publish to Natss
		ch := getSubject(channel)
		message, err := json.Marshal(m)
		if err != nil {
			logger.Errorf("Error during marshaling of the message: %v", err)
			return err
		}
		if s.natssConn == nil {
			return fmt.Errorf("No Connection to NATS")
		}
		if err := stanutil.Publish(s.natssConn, ch, &m.Payload, logger); err != nil {
			logger.Errorf("Error during publish: %v", err)
			if err.Error() == stan.ErrConnectionClosed.Error() {
				logger.Error("Connection to NATS has been lost, attempting to reconnect.")
				// Informing SubscriptionsSupervisor to re-establish connection to NATS
				s.connect <- struct{}{}
				return err
			}
			return err
		}
		logger.Infof("Published [%s] : '%s'", channel.String(), m.Headers)
		return nil
	}
}

func (s *SubscriptionsSupervisor) Start(stopCh <-chan struct{}) error {
	// Trigger Connect to establish connection with NATS
	s.connect <- struct{}{}
	s.receiver.Start(stopCh)
	return nil
}

func (s *SubscriptionsSupervisor) connectWithRetry() {
	// re-attempting evey 60 seconds until the connection is established.
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	s.subscriptionsMux.Lock()
	defer s.subscriptionsMux.Unlock()
	for {
		nConn, err := stanutil.Connect(clusterchannelprovisioner.ClusterId, clientID, s.natssURL, s.logger.Sugar())
		if err == nil {
			s.natssConn = nConn
			s.natssConnInProgress = false
			return
		}
		s.logger.Sugar().Errorf("Connect() failed with error: %+v, retrying in 60 seconds", err)
		select {
		case <-ticker.C:
			continue
		}
	}
}

// Connect is called for initial connection as well as after every disconnect
func (s *SubscriptionsSupervisor) Connect() {
	for {
		select {
		case <-s.connect:
			// Case for initial connection to NATSS
			if s.natssConn == nil && !s.natssConnInProgress {
				// Setting up InProgress to true to prevent recursion
				s.subscriptionsMux.Lock()
				s.natssConnInProgress = true
				s.subscriptionsMux.Unlock()
				go s.connectWithRetry()
				continue
			}
			// Case when the connection to NATSS was lost
			if s.natssConn != nil && (*s.natssConn).NatsConn().IsClosed() && !s.natssConnInProgress {
				// Setting up InProgress to true to prevent recursion
				s.subscriptionsMux.Lock()
				s.natssConn = nil
				s.natssConnInProgress = true
				s.subscriptionsMux.Unlock()
				go s.connectWithRetry()
			}
		}
	}
}

func (s *SubscriptionsSupervisor) UpdateSubscriptions(channel *eventingv1alpha1.Channel, isFinalizer bool) error {
	s.subscriptionsMux.Lock()
	defer s.subscriptionsMux.Unlock()

	cRef := provisioners.ChannelReference{Namespace: channel.Namespace, Name: channel.Name}

	if channel.Spec.Subscribable == nil || isFinalizer {
		s.logger.Sugar().Infof("Empty subscriptions for channel Ref: %v; unsubscribe all active subscriptions, if any", cRef)
		chMap, ok := s.subscriptions[cRef]
		if !ok {
			// nothing to do
			s.logger.Sugar().Infof("No channel Ref %v found in subscriptions map", cRef)
			return nil
		}
		for sub := range chMap {
			s.unsubscribe(cRef, sub)
		}
		delete(s.subscriptions, cRef)
		return nil
	}

	subscriptions := channel.Spec.Subscribable.Subscribers
	activeSubs := make(map[subscriptionReference]bool) // it's logically a set

	chMap, ok := s.subscriptions[cRef]
	if !ok {
		chMap = make(map[subscriptionReference]*stan.Subscription)
		s.subscriptions[cRef] = chMap
	}
	for _, sub := range subscriptions {
		// check if the subscription already exist and do nothing in this case
		subRef := newSubscriptionReference(sub)
		if _, ok := chMap[subRef]; ok {
			activeSubs[subRef] = true
			s.logger.Sugar().Infof("Subscription: %v already active for channel: %v", sub, cRef)
			continue
		}
		// subscribe
		natssSub, err := s.subscribe(cRef, subRef)
		if err != nil {
			return err
		}
		chMap[subRef] = natssSub
		activeSubs[subRef] = true
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

func (s *SubscriptionsSupervisor) subscribe(channel provisioners.ChannelReference, subscription subscriptionReference) (*stan.Subscription, error) {
	s.logger.Info("Subscribe to channel:", zap.Any("channel", channel), zap.Any("subscription", subscription))

	mcb := func(msg *stan.Msg) {
		s.logger.Sugar().Infof("NATSS message received from subject: %v; sequence: %v; timestamp: %v, data: %s", msg.Subject, msg.Sequence, msg.Timestamp, string(msg.Data))
		message := provisioners.Message{}
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			s.logger.Error("Failed to unmarshal message: ", zap.Error(err))
			return
		}
		if err := s.dispatcher.DispatchMessage(&message, subscription.SubscriberURI, subscription.ReplyURI, provisioners.DispatchDefaults{Namespace: subscription.Namespace}); err != nil {
			s.logger.Error("Failed to dispatch message: ", zap.Error(err))
			return
		}
		if err := msg.Ack(); err != nil {
			s.logger.Error("Failed to acknowledge message: ", zap.Error(err))
		}
	}
	// subscribe to a NATSS subject
	ch := getSubject(channel)
	sub := subscription.String()
	if s.natssConn == nil {
		return nil, fmt.Errorf("No Connection to NATS")
	}
	natssSub, err := (*s.natssConn).Subscribe(ch, mcb, stan.DurableName(sub), stan.SetManualAckMode(), stan.AckWait(1*time.Minute))
	if err != nil {
		s.logger.Error(" Create new NATSS Subscription failed: ", zap.Error(err))
		if err.Error() == stan.ErrConnectionClosed.Error() {
			s.logger.Error("Connection to NATS has been lost, attempting to reconnect.")
			// Informing SubscriptionsSupervisor to re-establish connection to NATS
			s.connect <- struct{}{}
			return nil, err
		}
		return nil, err
	}
	s.logger.Sugar().Infof("NATSS Subscription created: %+v", natssSub)
	return &natssSub, nil
}

// should be called only while holding subscriptionsMux
func (s *SubscriptionsSupervisor) unsubscribe(channel provisioners.ChannelReference, subscription subscriptionReference) error {
	s.logger.Info("Unsubscribe from channel:", zap.Any("channel", channel), zap.Any("subscription", subscription))

	if stanSub, ok := s.subscriptions[channel][subscription]; ok {
		// delete from NATSS
		if err := (*stanSub).Unsubscribe(); err != nil {
			s.logger.Error("Unsubscribing NATS Streaming subscription failed: ", zap.Error(err))
			return err
		}
		delete(s.subscriptions[channel], subscription)
	}
	return nil
}

func getSubject(channel provisioners.ChannelReference) string {
	return channel.Name + "." + channel.Namespace
}
