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
	"os"
	"testing"
	"time"

	"github.com/knative/eventing/contrib/natss/pkg/stanutil"
	"github.com/knative/eventing/contrib/natss/pkg/controller/clusterchannelprovisioner"
	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/nats-io/nats-streaming-server/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	natssTestURL = "nats://localhost:4222"

	ccpName = "natss"

	cNamespace = "test-namespace"
	cName      = "test-channel"
	cUID       = "test-uid"
)

var (
	clusterID   = clusterchannelprovisioner.ClusterId
	logger      *zap.SugaredLogger
	core        zapcore.Core
	observed    *observer.ObservedLogs
	testLogger  *zap.Logger
	s           *SubscriptionsSupervisor
	subscribers = &v1alpha1.Subscribable{
		Subscribers: []v1alpha1.ChannelSubscriberSpec{
			{
				Ref: &corev1.ObjectReference{
					Name:      "sub-name1",
					Namespace: "sub-namespace1",
					UID:       "sub-uid1",
				},
			},
			{
				Ref: &corev1.ObjectReference{
					Name:      "sub-name2",
					Namespace: "sub-namespace2",
					UID:       "sub-uid2",
				},
			},
		},
	}
)

func TestMain(m *testing.M) {
	logger = provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
	defer logger.Sync()

	core, observed = observer.New(zapcore.InfoLevel)
	testLogger = zap.New(core)
	// Start NATSS.
	stanServer, err := startNatss()
	if err != nil {
		logger.Fatalf("Cannot start NATSS: %v", err)
	}
	defer stopNatss(stanServer)
	// Create and start Dispatcher.
	s, err = NewDispatcher(natssTestURL, testLogger)
	if err != nil {
		logger.Fatalf("Unable to create NATSS dispatcher: %v", err)
	}
	// Starting Connect to establish connection with NATS
	go s.Connect()
	// forcing to establish connection to NATS before any tests
	s.connect <- struct{}{}
	ready := false
	ticker := time.NewTicker(time.Second * 5)
	expire := time.NewTimer(time.Second * 120)
	for !ready {
		select {
		case <-ticker.C:
			s.natssConnMux.Lock()
			if s.natssConn != nil {
				ready = true
			}
			s.natssConnMux.Unlock()
		case <-expire.C:
			logger.Fatalf("Failed to connect to NATSS!")
		}
	}
	stopCh := make(chan struct{})
	defer close(stopCh)
	go func() {
		s.Start(stopCh)
		if s.natssConn == nil {
			logger.Fatalf("Failed to connect to NATSS!")
		}
	}()
	os.Exit(m.Run())
}

func TestSubscribeUnsubscribe(t *testing.T) {
	logger.Info("TestSubscribeUnsubscribe()")

	cRef := provisioners.ChannelReference{Namespace: "test_namespace", Name: "test_channel"}
	sRef := subscriptionReference{Name: "sub_name_2", Namespace: "sub_namespace_2", SubscriberURI: "", ReplyURI: ""}

	// subscribe to a channel
	if _, err := s.subscribe(cRef, sRef); err != nil {
		t.Errorf("Subscribe to NATSS failed: %v", err)
	}
	if err := s.unsubscribe(cRef, sRef); err != nil {
		t.Errorf("Close subscription to NATSS failed: %v", err)
	}
}

func TestMalformedMessage(t *testing.T) {
	logger.Info("TestMalformedMessage()")

	cRef := provisioners.ChannelReference{Namespace: "test_namespace", Name: "test_channel"}
	sRef := subscriptionReference{Name: "sub_name", Namespace: "sub_namespace", SubscriberURI: "", ReplyURI: ""}

	// subscribe to a channel
	if _, err := s.subscribe(cRef, sRef); err != nil {
		t.Errorf("Subscribe to NATSS failed: %v", err)
	}
	defer func() {
		if err := s.unsubscribe(cRef, sRef); err != nil {
			t.Errorf("Close subscription to NATSS failed: %v", err)
		}
	}()

	m := &provisioners.Message{
		Headers: map[string]string{"header1": "value1", "header2": "value2"},
		Payload: []byte{'1', '2', '3', '4', '5'},
	}
	ch := getSubject(cRef)
	message, err := json.Marshal(m)
	if err != nil {
		t.Errorf("Error during marshaling of the message: %v", err)
	}
	// Corrupting message so it would fail Unmarshal
	b := message[:len(message)-3]
	if err := stanutil.Publish(s.natssConn, ch, &b, testLogger.Sugar()); err != nil {
		logger.Errorf("Error during publish: %v", err)
		t.Errorf("Error during publish: %v", err)
	}
	// Need to wait until the messages reaches mcb() where Unmarshal takes place
	timeout := time.After(20 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	for {
		if observed.FilterMessageSnippet("Failed to unmarshal message:").Len() != 0 {
			t.Log("Expected error message is present.")
			break
		}
		select {
		case <-ticker.C:
			continue
		case <-timeout:
			t.Errorf("Timoeout waiting for the expected error message, test TestMalformedMessage failed")
		}
	}
}

func TestUpdateSubscriptions(t *testing.T) {
	logger.Info("TestUpdateSubscriptions()")

	c := makeChannelWithSubscribers()
	if err := s.UpdateSubscriptions(c, false); err != nil {
		t.Errorf("UpdateSubscriptions failed: %v", err)
	}

	cRef := provisioners.ChannelReference{c.Namespace, c.Name}
	chMap, ok := s.subscriptions[cRef]
	if !ok {
		t.Error("No channel map found")
	}

	// check the subscriptions
	if len(chMap) != len(subscribers.Subscribers) {
		t.Errorf("Wrong channel map length: %v", chMap)
	}

	// Unsubscribe for all subscriptions
	for sub := range chMap {
		if err := s.unsubscribe(cRef, sub); err != nil {
			t.Errorf("Unsubscribe failed for subscription: %v, error: %v", sub, err)
		}
	}
}

func startNatss() (*server.StanServer, error) {
	logger.Infof("Start NATSS")
	var (
		err        error
		stanServer *server.StanServer
	)
	for i := 0; i < 10; i++ {
		if stanServer, err = server.RunServer(clusterID); err != nil {
			logger.Errorf("Start NATSS failed: %+v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	return stanServer, err
}

func stopNatss(server *server.StanServer) {
	logger.Info("Stop NATSS")
	server.Shutdown()
}

func makeChannel() *eventingv1alpha1.Channel {
	c := &eventingv1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Channel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cNamespace,
			Name:      cName,
			UID:       cUID,
		},
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &corev1.ObjectReference{
				Name: ccpName,
			},
		},
	}
	c.Status.InitializeConditions()
	return c
}

func makeChannelWithSubscribers() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Spec.Subscribable = subscribers
	return c
}
