package testing

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go"
)

type TestCloudEventsClient struct {
	Sent []cloudevents.Event
}

var _ cloudevents.Client = (*TestCloudEventsClient)(nil)

func (c *TestCloudEventsClient) Send(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, error) {
	// TODO: improve later.
	c.Sent = append(c.Sent, event)
	return nil, nil
}

func (c *TestCloudEventsClient) StartReceiver(ctx context.Context, fn interface{}) error {
	// TODO: improve later.
	<-ctx.Done()
	return nil
}

func (c *TestCloudEventsClient) Reset() {
	c.Sent = make([]cloudevents.Event, 0)
}

func NewTestClient() *TestCloudEventsClient {

	c := &TestCloudEventsClient{
		Sent: make([]cloudevents.Event, 0),
	}

	return c
}
