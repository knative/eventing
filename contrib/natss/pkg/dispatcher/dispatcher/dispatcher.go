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

	"github.com/knative/eventing/contrib/natss/pkg/stanutil"
	"github.com/knative/eventing/pkg/provisioners"
	stan "github.com/nats-io/go-nats-streaming"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/event"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

// The SubscriptionSupervisor handles both publishing and subscribing to NATS subjects.
//
// Publishing - simply uses the core knative eventing MessageReceiver with `createReceiverFunction` to
// receive events from the knative Channel and forward them to the appropriate NATS subject.
//
// Subscribing - SubscriptionSupervisor reconciles Channel resources with corresponding NATS subject
// subscriptions, subscribing and unsubscribing from subjects as needed.
//
// NATS client disconnects must be carefully handled. Every call using the NATS client can potentially
// error due to a disconnect. When this happens, we signal the connect chan to reconnect the client.
// Once we reconnect we additionally signal the reconcileChan for each Channel in order
// to resubscribe to each Channel through the normal reconciliation loop.
// (This is triggered by the Watch setup in channel/controller.go)
const (
	clientID = "knative-natss-dispatcher"
	// maxElements defines a maximum number of outstanding re-connect requests
	maxElements = 10
)

var (
	// retryInterval defines delay in seconds for the next attempt to reconnect to NATSS streaming server
	retryInterval = 1 * time.Second
)

// SubscriptionsSupervisor manages the state of NATS Streaming subscriptions
type SubscriptionsSupervisor struct {
	logger *zap.Logger

	receiver   *provisioners.MessageReceiver
	dispatcher *provisioners.MessageDispatcher

	subscriptionsMux sync.Mutex
	// subscriptions maintains the current active NATS subject subscription for a given channel.
	// This map is cleared on NATS client reconnect in order to allow for resubscription.
	subscriptions map[provisioners.ChannelReference]map[subscriptionReference]*stan.Subscription

	connect   chan struct{}
	natssURL  string
	clusterID string
	// natConnMux is used to protect natssConn and natssConnInProgress during
	// the transition from not connected to connected states.
	natssConnMux        sync.Mutex
	natssConn           *stan.Conn
	natssConnInProgress bool

	reconcileChan chan event.GenericEvent
	// reconcilers is a map from ChannelReference to a function that will send a GenericEvent into
	// the reconcileChan on NATS client reconnect. This triggers the reconciliation loop so that
	// we resubscribe to the NATS subject. The func accepts a `stopCh`.
	reconcilers map[provisioners.ChannelReference]func(<-chan struct{})
}

// NewDispatcher returns a new SubscriptionsSupervisor.
func NewDispatcher(natssURL, clusterID string, logger *zap.Logger) (*SubscriptionsSupervisor, error) {
	d := &SubscriptionsSupervisor{
		logger:        logger,
		dispatcher:    provisioners.NewMessageDispatcher(logger.Sugar()),
		connect:       make(chan struct{}, maxElements),
		natssURL:      natssURL,
		clusterID:     clusterID,
		subscriptions: make(map[provisioners.ChannelReference]map[subscriptionReference]*stan.Subscription),
		reconcileChan: make(chan event.GenericEvent),
		reconcilers:   make(map[provisioners.ChannelReference]func(<-chan struct{})),
	}
	d.receiver = provisioners.NewMessageReceiver(createReceiverFunction(d, logger.Sugar()), logger.Sugar())

	return d, nil
}

func (s *SubscriptionsSupervisor) ReconcileChan() <-chan event.GenericEvent {
	return s.reconcileChan
}

func (s *SubscriptionsSupervisor) signalReconnect() {
	select {
	case s.connect <- struct{}{}:
		// Sent.
	default:
		// The Channel is already full, so a reconnection attempt will occur.
	}
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
		s.natssConnMux.Lock()
		currentNatssConn := s.natssConn
		s.natssConnMux.Unlock()
		if currentNatssConn == nil {
			return fmt.Errorf("No Connection to NATSS")
		}
		if err := stanutil.Publish(currentNatssConn, ch, &message, logger); err != nil {
			logger.Errorf("Error during publish: %v", err)
			if err.Error() == stan.ErrConnectionClosed.Error() {
				logger.Error("Connection to NATSS has been lost, attempting to reconnect.")
				// Informing SubscriptionsSupervisor to re-establish connection to NATSS.
				s.signalReconnect()
				return err
			}
			return err
		}
		logger.Infof("Published [%s] : '%s'", channel.String(), m.Headers)
		return nil
	}
}

func (s *SubscriptionsSupervisor) Start(stopCh <-chan struct{}) error {
	// Starting Connect to establish connection with NATS
	go s.Connect(stopCh)
	// Trigger Connect to establish connection with NATS
	s.signalReconnect()
	return s.receiver.Start(stopCh)
}

func (s *SubscriptionsSupervisor) connectWithRetry(stopCh <-chan struct{}) {
	// re-attempting evey 1 second until the connection is established.
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	for {
		nConn, err := stanutil.Connect(s.clusterID, clientID, s.natssURL, s.logger.Sugar(), func(reason error) {
			s.logger.Sugar().Errorf("Lost connection: %+v, reconnecting.", reason)
			s.signalReconnect()
		})
		if err == nil {
			// Locking here in order to reduce time in locked state.
			s.natssConnMux.Lock()
			s.natssConn = nConn
			s.natssConnInProgress = false
			s.natssConnMux.Unlock()

			s.signalReconcile(stopCh)
			return
		}
		s.logger.Sugar().Errorf("Connect() failed with error: %+v, retrying in %s", err, retryInterval.String())
		select {
		case <-ticker.C:
			continue
		case <-stopCh:
			return
		}
	}
}

func (s *SubscriptionsSupervisor) signalReconcile(stopCh <-chan struct{}) {
	s.subscriptionsMux.Lock()
	defer s.subscriptionsMux.Unlock()

	// Clear subscriptions to force resubscribe
	s.subscriptions = make(map[provisioners.ChannelReference]map[subscriptionReference]*stan.Subscription)

	// Capture + clear reonciliation
	reconcilers := s.reconcilers
	s.reconcilers = make(map[provisioners.ChannelReference]func(<-chan struct{}))

	// Run in a goroutine since signalling can block
	go func() {
		for _, reconciler := range reconcilers {
			reconciler(stopCh)
		}
	}()
}

// Connect is called for initial connection as well as after every disconnect
func (s *SubscriptionsSupervisor) Connect(stopCh <-chan struct{}) {
	for {
		select {
		case <-s.connect:
			s.natssConnMux.Lock()
			currentConnProgress := s.natssConnInProgress
			if currentConnProgress {
				s.natssConnMux.Unlock()
			} else {
				s.natssConnInProgress = true
				s.natssConnMux.Unlock()
				go s.connectWithRetry(stopCh)
			}
		case <-stopCh:
			return
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
		s.deleteChannel(cRef)
		return nil
	}

	s.reconcilers[cRef] = func(stopCh <-chan struct{}) {
		select {
		case s.reconcileChan <- event.GenericEvent{
			Meta:   channel.GetObjectMeta(),
			Object: channel,
		}:
		case <-stopCh:
		}
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
		s.deleteChannel(cRef)
	}
	return nil
}

func (s *SubscriptionsSupervisor) deleteChannel(channel provisioners.ChannelReference) {
	delete(s.subscriptions, channel)
	delete(s.reconcilers, channel)
}

func (s *SubscriptionsSupervisor) subscribe(channel provisioners.ChannelReference, subscription subscriptionReference) (*stan.Subscription, error) {
	s.logger.Info("Subscribe to channel:", zap.Any("channel", channel), zap.Any("subscription", subscription))

	mcb := func(msg *stan.Msg) {
		message := provisioners.Message{}
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			s.logger.Error("Failed to unmarshal message: ", zap.Error(err))
			return
		}
		s.logger.Sugar().Infof("NATSS message received from subject: %v; sequence: %v; timestamp: %v, headers: '%s'", msg.Subject, msg.Sequence, msg.Timestamp, message.Headers)
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
	s.natssConnMux.Lock()
	currentNatssConn := s.natssConn
	s.natssConnMux.Unlock()
	if currentNatssConn == nil {
		return nil, fmt.Errorf("No Connection to NATSS")
	}
	natssSub, err := (*currentNatssConn).Subscribe(ch, mcb, stan.DurableName(sub), stan.SetManualAckMode(), stan.AckWait(1*time.Minute))
	if err != nil {
		s.logger.Error(" Create new NATSS Subscription failed: ", zap.Error(err))
		if err.Error() == stan.ErrConnectionClosed.Error() {
			s.logger.Error("Connection to NATSS has been lost, attempting to reconnect.")
			// Informing SubscriptionsSupervisor to re-establish connection to NATS
			s.signalReconnect()
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
			s.logger.Error("Unsubscribing NATSS Streaming subscription failed: ", zap.Error(err))
			return err
		}
		delete(s.subscriptions[channel], subscription)
	}
	return nil
}

func getSubject(channel provisioners.ChannelReference) string {
	return channel.Name + "." + channel.Namespace
}
