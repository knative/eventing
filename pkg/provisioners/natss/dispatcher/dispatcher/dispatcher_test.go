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
	"os"
	"testing"
	"time"

	"github.com/knative/eventing/pkg/provisioners"
	"github.com/nats-io/nats-streaming-server/server"
	"go.uber.org/zap"
)

const (
	clusterId      = "knative-nats-streaming"
	natssTestUrl   = "nats://localhost:4222"
)

var (
	logger *zap.SugaredLogger
)

func TestMain(m *testing.M) {
	logger = provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
	defer logger.Sync()

	stanServer, err := startNatss()
	if err != nil {
		panic(err)
	}
	defer stopNatss(stanServer)

	// run the unit tests
	retCode := m.Run()

	os.Exit(retCode)
}

func TestCreateDeleteSubscriptionSupervisor(t *testing.T) {
	logger.Info("Creating Subscription Supervisor")
	logger.Info("Dispatcher starting...")
	s, err := NewDispatcher(natssTestUrl, logger.Desugar())
	if err != nil {
		t.Fatalf("Unable to create NATSS dispatcher.")
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	go func() {
		s.Start(stopCh)
		if s.natssConn == nil {
			t.Fatal("Failed to connect to NATSS")
		}
	}()

	// subscribe to a channel
	cRef := provisioners.ChannelReference{Namespace: "test_namespace", Name: "test_channel"}
	sRef := subscriptionReference {Name: "sub_name", Namespace: "sub_namespace", SubscriberURI: "", ReplyURI: ""}

	if _, err := s.subscribe(cRef, sRef); err != nil {
		t.Errorf("Subscribe to NATSS failed: %v", err)
	}
	if err = s.unsubscribe(cRef, sRef); err != nil {
		t.Errorf("Close subscription to NATSS failed: %v", err)
	}
}

func startNatss() (*server.StanServer, error) {
	logger.Infof("Start NATSS")
	var err error
	var stanServer *server.StanServer
	for i := 0; i < 10; i++ {
		if stanServer, err = server.RunServer(clusterId); err != nil {
			logger.Errorf("Start NATSS failed: %+v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	return stanServer, nil
}

func stopNatss(server *server.StanServer) {
	logger.Infof("Stop NATSS")
	server.Shutdown()
}
