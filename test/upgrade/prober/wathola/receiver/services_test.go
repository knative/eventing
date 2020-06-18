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
	"k8s.io/apimachinery/pkg/util/wait"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/event"
	"knative.dev/eventing/test/upgrade/prober/wathola/sender"

	"os"
	"testing"
	"time"
)

func TestReceiverReceive(t *testing.T) {
	// given
	e1 := sender.NewCloudEvent(event.Step{Number: 42}, event.StepType)
	e2 := sender.NewCloudEvent(event.Step{Number: 43}, event.StepType)
	f := sender.NewCloudEvent(event.Finished{Count: 2}, event.FinishedType)

	instance := New()
	port := freeport.GetPort()
	config.Instance.Receiver.Port = port
	go instance.Receive()
	cancel := <-Canceling
	defer cancel()
	assert.NoError(t, testlib.WaitForReadiness(port, config.Log))

	// when
	sendEvent(t, e1, port)
	sendEvent(t, e2, port)
	sendEvent(t, f, port)

	// then
	rr := instance.(*receiver)
	assert.NoError(t, waitUntilFinished(rr))
	assert.Equal(t, 2, rr.step.Count())
	assert.Equal(t, event.Success, rr.finished.State())
}

func TestMain(m *testing.M) {
	config.Instance.Receiver.Teardown.Duration = time.Millisecond
	exitcode := m.Run()
	os.Exit(exitcode)
}

func waitUntilFinished(r *receiver) error {
	return wait.PollImmediate(5*time.Millisecond, 30*time.Second, func() (done bool, err error) {
		state := r.finished.State()
		return state != event.Active, nil
	})
}

func sendEvent(t *testing.T, e cloudevents.Event, port int) {
	url := fmt.Sprintf("http://localhost:%v/", port)
	err := sender.SendEvent(e, url)
	assert.NoError(t, err)
}
