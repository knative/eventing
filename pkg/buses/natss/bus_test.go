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

package natss

import (
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/buses"
	"github.com/nats-io/nats-streaming-server/server"
	"go.uber.org/zap"
	"os"
	"testing"
	"time"
)

const (
	clusterId      = "knative-nats-streaming"
	natssTestUrl   = "nats://localhost:4222"
	messagePayload = "test-message-1"
)

var (
	logger *zap.SugaredLogger
	done   = make(chan string)
)

func TestMain(m *testing.M) {
	logger = buses.NewBusLoggerFromConfig(buses.NewLoggingConfig())
	defer logger.Sync()

	stanServer, err := startNatss()
	if err != nil {
		panic(err)
	}
	defer stopNatss(stanServer)

	retCode := m.Run()

	os.Exit(retCode)
}

func TestNatss(t *testing.T) {
	ref := buses.NewBusReferenceFromNames("NATSS_TEST_NAME", "NATSS_TEST_NAMESPACE")
	opts := &buses.BusOpts{
		Logger:     logger,
		KubeConfig: "",
		MasterURL:  "",
	}

	busProv, err := NewNatssBusProvisioner(ref, setNewTestBusProvisioner(t, opts))
	if err != nil {
		t.Fatalf("Unexpected error from NewNatssBusProvisioner: %v", err)
	}

	busDisp, err := NewNatssBusDispatcher(ref, setNewTestBusDispatcher(t, opts))
	if err != nil {
		t.Fatalf("Unexpected error from NewNatssBusDispatcher: %v", err)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	busProv.Run(1, stopCh, "test-provisioner-natss")
	busDisp.Run(1, stopCh, "test-dispatcher-natss")

	// create a dummy channel
	channel := buses.ChannelReference{"default", "test-channel"}
	busProv.provEventHandler.ProvisionFunc(channel, make(buses.ResolvedParameters))

	// create a dummy subscription
	subscription := buses.SubscriptionReference{"default", "test-subscription"}
	busDisp.dispEventHandler.SubscribeFunc(channel, subscription, make(buses.ResolvedParameters))

	// send a message ....
	message := buses.Message{make(map[string]string), []byte(messagePayload)}
	busDisp.dispEventHandler.ReceiveMessageFunc(channel, &message)

	// wait for subscriber to respond
	select {
	case payload := <-done:
		logger.Info("Subscriber finished")
		if diff := cmp.Diff(messagePayload, payload); diff != "" {
			t.Errorf("Unexpected message payload (-want, +got): %v", diff)
		}
	case <-time.After(5 * time.Second):
		t.Error("Subscriber timeout")
	}

	// unsubscribe
	busDisp.dispEventHandler.UnsubscribeFunc(channel, subscription)
}

func startNatss() (*server.StanServer, error) {
	return server.RunServer(clusterId)
}

func stopNatss(server *server.StanServer) {
	server.Shutdown()
}

// set dummy provisioner
func setNewTestBusProvisioner(t *testing.T, opts *buses.BusOpts) SetupNatssBus {
	return func(b *NatssBus) error {
		b.natssUrl = natssTestUrl
		b.provisioner = newDummyProvisioner(opts.Logger)
		b.logger = opts.Logger
		return nil
	}
}

func newDummyProvisioner(logger *zap.SugaredLogger) buses.BusProvisioner {
	var b = &dummyBusProvisioner{
		logger: logger,
	}
	return b
}

type dummyBusProvisioner struct {
	logger *zap.SugaredLogger
}

func (m *dummyBusProvisioner) Run(threadiness int, stopCh <-chan struct{}) {}

//set dummy dispatcher
func setNewTestBusDispatcher(t *testing.T, opts *buses.BusOpts) SetupNatssBus {
	return func(b *NatssBus) error {
		b.natssUrl = natssTestUrl
		b.dispatcher = newDummyDispatcher(opts.Logger)
		b.logger = opts.Logger
		return nil
	}
}

func newDummyDispatcher(logger *zap.SugaredLogger) buses.BusDispatcher {
	var b = &dummyBusDispatcher{
		logger: logger,
	}
	return b
}

type dummyBusDispatcher struct {
	logger *zap.SugaredLogger
}

func (m *dummyBusDispatcher) Run(threadiness int, stopCh <-chan struct{}) {}

func (m *dummyBusDispatcher) DispatchMessage(subscription buses.SubscriptionReference, message *buses.Message) error {
	payload := string(message.Payload)
	m.logger.Infof("dummyBusDispatcher() - received message: %s", payload)
	done <- payload
	return nil
}
