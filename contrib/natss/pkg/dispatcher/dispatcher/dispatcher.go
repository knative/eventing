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
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/knative/eventing/contrib/natss/pkg/stanutil"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners"
	stan "github.com/nats-io/go-nats-streaming"
	"go.uber.org/zap"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

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
	subscriptions    map[provisioners.ChannelReference]map[subscriptionReference]*stan.Subscription

	connect   chan struct{}
	natssURL  string
	clusterID string
	// natConnMux is used to protect natssConn and natssConnInProgress during
	// the transition from not connected to connected states.
	natssConnMux        sync.Mutex
	natssConn           *stan.Conn
	natssConnInProgress bool

	hostToChannelMap atomic.Value
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
	}
	d.setHostToChannelMap(map[string]provisioners.ChannelReference{})
	receiver, err := provisioners.NewMessageReceiver(
		createReceiverFunction(d, logger.Sugar()),
		logger.Sugar(),
		provisioners.ResolveChannelFromHostHeader(provisioners.ResolveChannelFromHostFunc(d.getChannelReferenceFromHost)))
	if err != nil {
		return nil, err
	}
	d.receiver = receiver
	return d, nil
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
		nConn, err := stanutil.Connect(s.clusterID, clientID, s.natssURL, s.logger.Sugar())
		if err == nil {
			// Locking here in order to reduce time in locked state.
			s.natssConnMux.Lock()
			s.natssConn = nConn
			s.natssConnInProgress = false
			s.natssConnMux.Unlock()
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

// Connect is called for initial connection as well as after every disconnect
func (s *SubscriptionsSupervisor) Connect(stopCh <-chan struct{}) {
	for {
		select {
		case <-s.connect:
			s.natssConnMux.Lock()
			currentConnProgress := s.natssConnInProgress
			s.natssConnMux.Unlock()
			if !currentConnProgress {
				// Case for lost connectivity, setting InProgress to true to prevent recursion
				s.natssConnMux.Lock()
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
		message := provisioners.Message{}
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			s.logger.Error("Failed to unmarshal message: ", zap.Error(err))
			return
		}
		s.logger.Sugar().Infof("NATSS message received from subject: %v; sequence: %v; timestamp: %v, headers: '%s'", msg.Subject, msg.Sequence, msg.Timestamp, message.Headers)
		if err := s.dispatcher.DispatchMessage(&message, subscription.SubscriberURI, subscription.ReplyURI, provisioners.DispatchDefaults{Namespace: channel.Namespace}); err != nil {
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

func (s *SubscriptionsSupervisor) getHostToChannelMap() map[string]provisioners.ChannelReference {
	return s.hostToChannelMap.Load().(map[string]provisioners.ChannelReference)
}

func (s *SubscriptionsSupervisor) setHostToChannelMap(hcMap map[string]provisioners.ChannelReference) {
	s.hostToChannelMap.Store(hcMap)
}

// UpdateHostToChannelMap will be called from the controller that watches natss channels.
// It will update internal hostToChannelMap which is used to resolve the hostHeader of the
// incoming request to the correct ChannelReference in the receiver function.
func (s *SubscriptionsSupervisor) UpdateHostToChannelMap(ctx context.Context, chanList []eventingv1alpha1.Channel) error {
	hostToChanMap, err := provisioners.NewHostNameToChannelRefMap(chanList)
	if err != nil {
		logging.FromContext(ctx).Info("UpdateHostToChannelMap: Error occured when creating the new hostToChannel map.", zap.Error(err))
		return err
	}
	s.setHostToChannelMap(hostToChanMap)
	logging.FromContext(ctx).Info("hostToChannelMap updated successfully.")
	return nil
}

func (s *SubscriptionsSupervisor) getChannelReferenceFromHost(host string) (provisioners.ChannelReference, error) {
	chMap := s.getHostToChannelMap()
	cr, ok := chMap[host]
	if !ok {
		return cr, fmt.Errorf("Invalid HostName:%q. HostName not found in any of the watched natss channels", host)
	}
	return cr, nil
}
