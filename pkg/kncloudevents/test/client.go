/*
Copyright 2023 The Knative Authors

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
	nethttp "net/http"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"knative.dev/eventing/pkg/kncloudevents"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var _ kncloudevents.Client = (*Client)(nil)

func NewClient() *Client {
	return &Client{
		requests: make([]*inMemoryRequest, 0),
	}
}

type Client struct {
	delay       time.Duration
	requests    []*inMemoryRequest
	requestsMux sync.Mutex
}

func (c *Client) NewRequest(ctx context.Context, target duckv1.Addressable) (kncloudevents.Request, error) {
	c.requestsMux.Lock()
	defer c.requestsMux.Unlock()

	nethttpReqest, err := nethttp.NewRequestWithContext(ctx, "POST", target.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	request := inMemoryRequest{
		Target:     target,
		Request:    nethttpReqest,
		sentEvents: make([]event.Event, 0),
		delay:      c.delay,
	}
	c.requests = append(c.requests, &request)

	return &request, nil
}

// SentEvents returns all events sent within all requests of this client.
func (c *Client) SentEvents() []event.Event {
	events := make([]event.Event, 0)

	for _, req := range c.requests {
		events = append(events, req.SentEvents()...)
	}

	return events
}

var _ kncloudevents.Request = (*inMemoryRequest)(nil)

type inMemoryRequest struct {
	*nethttp.Request
	Target duckv1.Addressable

	delay      time.Duration
	sentEvents []event.Event
}

func (req *inMemoryRequest) HTTPRequest() *nethttp.Request {
	return req.Request
}

func (req *inMemoryRequest) BindEvent(ctx context.Context, event event.Event) error {
	message := binding.ToMessage(&event)
	defer message.Finish(nil)

	err := http.WriteRequest(ctx, message, req.HTTPRequest())
	return err
}

func (req *inMemoryRequest) Send() (*nethttp.Response, error) {
	if req.delay > 0 {
		time.Sleep(req.delay)
	}

	message := http.NewMessageFromHttpRequest(req.HTTPRequest())
	defer message.Finish(nil)

	event, err := binding.ToEvent(context.TODO(), message)
	if err == nil {
		req.sentEvents = append(req.sentEvents, *event)
	}

	return &nethttp.Response{
		StatusCode: nethttp.StatusOK,
	}, nil
}

func (req *inMemoryRequest) SendWithRetries(config *kncloudevents.RetryConfig) (*nethttp.Response, error) {
	return req.Send()
}

// SentEvents returns all events sent within this request.
func (req *inMemoryRequest) SentEvents() []event.Event {
	return req.sentEvents
}
