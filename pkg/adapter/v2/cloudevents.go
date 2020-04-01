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
package adapter

import (
	"context"
	"errors"

	"knative.dev/eventing/pkg/adapter/v2/metrics"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/source"
)

// NewCloudEventsClient returns a client that will apply the ceOverrides to
// outbound events and report outbound event counts.
func NewCloudEventsClient(target string, ceOverrides *duckv1.CloudEventOverrides, reporter source.StatsReporter) (cloudevents.Client, error) {
	pOpts := make([]http.Option, 0)
	if len(target) > 0 {
		pOpts = append(pOpts, cloudevents.WithTarget(target))
	}

	p, err := cloudevents.NewHTTP(pOpts...)
	if err != nil {
		return nil, err
	}

	ceClient, err := cloudevents.NewClientObserved(p, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		return nil, err
	}
	return &client{
		ceClient:    ceClient,
		ceOverrides: ceOverrides,
		reporter:    metrics.NewStatsReporterAdapter(reporter),
	}, nil
}

type client struct {
	ceClient    cloudevents.Client
	ceOverrides *duckv1.CloudEventOverrides
	reporter    metrics.StatsReporterAdapter
}

var _ cloudevents.Client = (*client)(nil)

// Send implements client.Send
func (c *client) Send(ctx context.Context, out event.Event) protocol.Result {
	c.applyOverrides(ctx, &out)
	res := c.ceClient.Send(ctx, out)
	return c.reporter.ReportCount(ctx, out, res)

}

// Request implements client.Request
func (c *client) Request(ctx context.Context, out event.Event) (*event.Event, protocol.Result) {
	c.applyOverrides(ctx, &out)

	resp, res := c.ceClient.Request(ctx, out)
	return resp, c.reporter.ReportCount(ctx, out, res)

}

// StartReceiver implements client.StartReceiver
func (c *client) StartReceiver(ctx context.Context, fn interface{}) error {
	return errors.New("not implemented")
}

func (c *client) applyOverrides(ctx context.Context, event *cloudevents.Event) {
	if c.ceOverrides != nil && c.ceOverrides.Extensions != nil {
		for n, v := range c.ceOverrides.Extensions {
			event.SetExtension(n, v)
		}
	}
}
