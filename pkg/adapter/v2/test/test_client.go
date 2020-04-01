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
	"sync"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type TestCloudEventsClient struct {
	lock sync.Mutex
	sent []cloudevents.Event
}

var _ cloudevents.Client = (*TestCloudEventsClient)(nil)

func (c *TestCloudEventsClient) Send(ctx context.Context, out event.Event) protocol.Result {
	c.lock.Lock()
	defer c.lock.Unlock()
	// TODO: improve later.
	c.sent = append(c.sent, out)
	return http.NewResult(200, "%w", protocol.ResultACK)
}

func (c *TestCloudEventsClient) Request(ctx context.Context, out event.Event) (*event.Event, protocol.Result) {
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
	c := &TestCloudEventsClient{
		sent: make([]cloudevents.Event, 0),
	}
	return c
}
