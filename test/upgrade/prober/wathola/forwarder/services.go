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

package forwarder

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/eventing/test/upgrade/prober/wathola/client"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/sender"

	"time"
)

var (
	log                = config.Log
	lastProgressReport = time.Now()
	Canceling          = make(chan context.CancelFunc, 1)
)

// New creates new forwarder
func New() Forwarder {
	config.ReadIfPresent()
	f := &forwarder{
		count: 0,
	}
	return f
}

func (f *forwarder) Forward() {
	port := config.Instance.Forwarder.Port
	client.Receive(port, Canceling, f.forwardEvent)
}

func (f *forwarder) forwardEvent(e cloudevents.Event) {
	target := config.Instance.Forwarder.Target
	log.Debugf("Forwarding event %v to %v", e.ID(), target)
	err := sender.SendEvent(e, target)
	if err != nil {
		log.Error(err)
	}
	f.count++
	f.reportProgress()
}

func (f *forwarder) reportProgress() {
	if lastProgressReport.Add(config.Instance.Receiver.Progress.Duration).Before(time.Now()) {
		lastProgressReport = time.Now()
		log.Infof("forwarded %v events", f.count)
	}
}

type forwarder struct {
	count int
}
