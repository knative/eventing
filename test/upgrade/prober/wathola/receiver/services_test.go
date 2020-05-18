/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package receiver

import (
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/event"
	"knative.dev/eventing/test/upgrade/prober/wathola/sender"

	"os"
	"testing"
	"time"
)

func TestReceiverReceive(t *testing.T) {
	// given
	e := sender.NewCloudEvent(event.Step{Number: 42}, event.StepType)
	f := sender.NewCloudEvent(event.Finished{Count: 1}, event.FinishedType)

	instance := New()
	port := freeport.GetPort()
	config.Instance.Receiver.Port = port
	go instance.Receive()
	cancel := <-Canceling
	defer cancel()

	// when
	sendEvent(t, e, port)
	sendEvent(t, f, port)

	// then
	rr := instance.(*receiver)
	assert.Equal(t, 1, rr.step.Count())
	assert.Equal(t, event.Success, rr.finished.State())
}

func TestMain(m *testing.M) {
	config.Instance.Receiver.Teardown.Duration = 20 * time.Millisecond
	exitcode := m.Run()
	os.Exit(exitcode)
}

func sendEvent(t *testing.T, e cloudevents.Event, port int) {
	url := fmt.Sprintf("http://localhost:%v/", port)
	err := sender.SendEvent(e, url)
	assert.NoError(t, err)
}
