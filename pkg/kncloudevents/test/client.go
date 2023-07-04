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
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"knative.dev/eventing/pkg/kncloudevents"
)

var _ kncloudevents.Client = (*FakeClient)(nil)

func NewFakeClient() *FakeClient {
	return &FakeClient{
		sentEvents: make([]event.Event, 0),
	}
}

type FakeClient struct {
	delay      time.Duration
	sentEvents []event.Event
}

// SentEvents returns all events sent within all requests of this client.
func (c *FakeClient) SentEvents() []event.Event {
	return c.sentEvents
}

func (c *FakeClient) Send(request *kncloudevents.Request) (*nethttp.Response, error) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	// get event from request to add to sentEvents
	message := http.NewMessageFromHttpRequest(request.HTTPRequest())
	defer message.Finish(nil)

	event, err := binding.ToEvent(context.TODO(), message)
	if err == nil {
		c.sentEvents = append(c.sentEvents, *event)
	}

	return &nethttp.Response{
		StatusCode: nethttp.StatusOK,
	}, nil
}

func (req *FakeClient) SendWithRetries(request *kncloudevents.Request, config *kncloudevents.RetryConfig) (*nethttp.Response, error) {
	return req.Send(request)
}
