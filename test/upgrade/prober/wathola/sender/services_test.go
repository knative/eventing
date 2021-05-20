/*
Copyright 2021 The Knative Authors

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

package sender_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/upgrade/prober/wathola/client"
	"knative.dev/eventing/test/upgrade/prober/wathola/sender"
)

func TestHTTPEventSender(t *testing.T) {
	sender.ResetEventSenders()
	ce := sender.NewCloudEvent(nil, "cetype")
	port := freeport.GetPort()
	canceling := make(chan context.CancelFunc, 1)
	events := make([]cloudevents.Event, 0, 1)
	go client.Receive(port, canceling, func(e cloudevents.Event) {
		events = append(events, e)
	})
	cancel := <-canceling
	waitForPort(t, port)
	err := sender.SendEvent(ce, fmt.Sprintf("http://localhost:%d", port))
	cancel()
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, ce, events[0])
}

func TestRegisterEventSender(t *testing.T) {
	sender.RegisterEventSender(testEventSender{})
	defer sender.ResetEventSenders()
	c := testConfig{
		topic:   "sample",
		servers: "example.org:9290",
		valid:   true,
	}
	ce := sender.NewCloudEvent(nil, "cetype")
	err := sender.SendEvent(ce, c)

	assert.NoError(t, err)
}

func TestUnsupportedEventSender(t *testing.T) {
	sender.RegisterEventSender(testEventSender{})
	defer sender.ResetEventSenders()
	ce := sender.NewCloudEvent(nil, "cetype")
	err := sender.SendEvent(ce, "https://example.org/")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, sender.ErrEndpointTypeNotSupported))
}

func waitForPort(t *testing.T, port int) {
	if err := wait.PollImmediate(time.Millisecond, 10*time.Second, func() (bool, error) {
		conn, conErr := net.Dial("tcp", fmt.Sprintf(":%d", port))
		defer func() {
			if conn != nil {
				assert.NoError(t, conn.Close())
			}
		}()
		if conErr != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("%v: port %d to be open", err, port)
	}
}

type testEventSender struct{}

type testConfig struct {
	topic   string
	servers string
	valid   bool
}

func (t testEventSender) Supports(endpoint interface{}) bool {
	switch endpoint.(type) {
	case testConfig:
		return true
	default:
		return false
	}
}

func (t testEventSender) SendEvent(ce cloudevents.Event, endpoint interface{}) error {
	cfg := endpoint.(testConfig)
	if cfg.valid {
		return nil
	}
	return fmt.Errorf("can't send %#v to %s server at %s topic",
		ce, cfg.servers, cfg.topic)
}
