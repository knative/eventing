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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type TestCloudEventsClient struct {
	lock          sync.Mutex
	sent          []cloudevents.Event
	delay         time.Duration
	resultSend    []protocol.Result
	resultRequest []struct {
		event  *event.Event
		result protocol.Result
	}
}

func (c *TestCloudEventsClient) CloseIdleConnections() {
}

type EventData struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

var eventData EventData

// Send_AppendResult will enqueue a response for the following Send call.
// For testing.
func (c *TestCloudEventsClient) Send_AppendResult(r protocol.Result) {
	c.resultSend = append(c.resultSend, r)
}

// Request_AppendResult will enqueue a response for the following Request call.
// For testing.
func (c *TestCloudEventsClient) Request_AppendResult(e *event.Event, r protocol.Result) {
	c.resultRequest = append(c.resultRequest, struct {
		event  *event.Event
		result protocol.Result
	}{event: e, result: r})
}

func (c *TestCloudEventsClient) Send(ctx context.Context, out event.Event) protocol.Result {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	// TODO: improve later.
	bytes, _ := json.Marshal(out)
	if err := json.Unmarshal(bytes, &eventData); err != nil {
		fmt.Printf("json unmarshal error %s:", err)
	}
	c.sent = append(c.sent, out)
	if eventData.Type == "unit.type" {
		return http.NewResult(200, "%w", protocol.ResultACK)
	} else if eventData.Type == "unit.retries" {
		var attempts []protocol.Result
		attempts = append(attempts, http.NewResult(500, "%w", protocol.ResultACK))
		return http.NewRetriesResult(http.NewResult(200, "%w", protocol.ResultACK), 1, time.Now(), attempts)
	} else if eventData.Type == "unit.wantErr" {
		return errors.New("totally not an http result")
	} else if eventData.Type == "unit.sendFail" {
		return http.NewResult(400, "%w", protocol.ResultNACK)
	} else if eventData.Type == "unit.nonHttpRetry" {
		var attempts []protocol.Result
		attempts = append(attempts, errors.New("totally not an http result"))
		return http.NewRetriesResult(http.NewResult(200, "%w", protocol.ResultACK), 1, time.Now(), attempts)
	}
	return http.NewResult(200, "%w", protocol.ResultACK)
}

func (c *TestCloudEventsClient) Request(ctx context.Context, out event.Event) (*event.Event, protocol.Result) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	// TODO: improve later.
	bytes, _ := json.Marshal(out)
	if err := json.Unmarshal(bytes, &eventData); err != nil {
		fmt.Printf("json unmarshal error %s:", err)
	}
	c.sent = append(c.sent, out)
	if eventData.Type == "unit.type" {
		return nil, http.NewResult(200, "%w", protocol.ResultACK)
	} else if eventData.Type == "unit.retries" {
		var attempts []protocol.Result
		attempts = append(attempts, http.NewResult(500, "%w", protocol.ResultACK))
		return nil, http.NewRetriesResult(http.NewResult(200, "%w", protocol.ResultACK), 1, time.Now(), attempts)
	} else if eventData.Type == "unit.wantErr" {
		return nil, errors.New("totally not an http result")
	} else if eventData.Type == "unit.sendFail" {
		return nil, http.NewResult(400, "%w", protocol.ResultNACK)
	}
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
	copy(r, c.sent)
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
