package testing

import (
	"context"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
)

type TestCloudEventsClient struct {
	lock sync.Mutex
	sent []cloudevents.Event
}

var _ cloudevents.Client = (*TestCloudEventsClient)(nil)

func (c *TestCloudEventsClient) Send(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	// TODO: improve later.
	c.sent = append(c.sent, event)
	return ctx, nil, nil
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
