/*
Copyright 2020 The Knative Authors

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
package test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type TestCloudEventsClient struct {
	lock  sync.Mutex
	sent  []cloudevents.Event
	delay time.Duration
}

type EventData struct {
	ID string `json:"id"`
	Type string `json:"type"`
}

var _ cloudevents.Client = (*TestCloudEventsClient)(nil)
var eventData EventData

func (c *TestCloudEventsClient) Send(ctx context.Context, out event.Event) protocol.Result {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	bytes, _ := json.Marshal(out)
	if err := json.Unmarshal(bytes, &eventData); err != nil {
		fmt.Print(err)
	}
	c.sent = append(c.sent, out)
	if eventData.Type == "unit.type" {
		return http.NewResult(200, "%w", protocol.ResultACK)
	} else if eventData.Type == "unit.retries" {
		var attempts []protocol.Result
		attempts = append(attempts, http.NewResult(500, "%w", protocol.ResultNACK))
		resp := http.NewRetriesResult(http.NewResult(200, "%w", protocol.ResultNACK), 1, time.Now(), attempts)
		return resp
	}
	return http.NewResult(200, "%w", protocol.ResultACK)
	// TODO: improve later.
}

func (c *TestCloudEventsClient) Request(ctx context.Context, out event.Event) (*event.Event, protocol.Result) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	// TODO: improve later.
	c.sent = append(c.sent, out)
	return nil, http.NewResult(200, "%w", protocol.ResultACK)
}

func (c *TestCloudEventsClient) StartReceiver(ctx context.Context, fn interface{}) error {
	// TODO: improve later.
	<-ctx.Done()
	return nil
}

func (c *TestCloudEventsClient) Reset() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.sent = make([]cloudevents.Event, 0)
}

func (c *TestCloudEventsClient) Sent() []cloudevents.Event {
	c.lock.Lock()
	defer c.lock.Unlock()
	r := make([]cloudevents.Event, len(c.sent))
	for i := range c.sent {
		r[i] = c.sent[i]
	}
	return r
}

func NewTestClient() *TestCloudEventsClient {
	return NewTestClientWithDelay(0)
}

func NewTestClientWithDelay(delay time.Duration) *TestCloudEventsClient {
	c := &TestCloudEventsClient{
		sent:  make([]cloudevents.Event, 0),
		delay: delay,
	}
	return c
}
