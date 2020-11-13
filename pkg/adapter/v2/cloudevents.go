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
	"fmt"
	nethttp "net/http"
	"net/url"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/source"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

// NewCloudEventsClient returns a client that will apply the ceOverrides to
// outbound events and report outbound event counts.
func NewCloudEventsClient(target string, ceOverrides *duckv1.CloudEventOverrides, reporter source.StatsReporter) (cloudevents.Client, error) {
	return newCloudEventsClientCRStatus(nil, target, ceOverrides, reporter, nil)
}
func NewCloudEventsClientCRStatus(env EnvConfigAccessor, reporter source.StatsReporter, crStatusEventClient *crstatusevent.CRStatusEventClient) (cloudevents.Client, error) {
	return newCloudEventsClientCRStatus(env, "", nil, reporter, crStatusEventClient)
}
func newCloudEventsClientCRStatus(env EnvConfigAccessor, target string, ceOverrides *duckv1.CloudEventOverrides, reporter source.StatsReporter,
	crStatusEventClient *crstatusevent.CRStatusEventClient) (cloudevents.Client, error) {

	if target == "" && env != nil {
		target = env.GetSink()
	}

	pOpts := make([]http.Option, 0)
	if len(target) > 0 {
		pOpts = append(pOpts, cloudevents.WithTarget(target))
	}
	pOpts = append(pOpts, cloudevents.WithRoundTripper(&ochttp.Transport{
		Propagation: tracecontextb3.TraceContextEgress,
	}))

	if env != nil {
		if sinkWait := env.GetSinktimeout(); sinkWait > 0 {
			pOpts = append(pOpts, setTimeOut(time.Duration(sinkWait)*time.Second))
		}
		var err error
		if ceOverrides == nil {
			ceOverrides, err = env.GetCloudEventOverrides()
			if err != nil {
				return nil, err
			}
		}
	}

	p, err := cloudevents.NewHTTP(pOpts...)
	if err != nil {
		return nil, err
	}

	ceClient, err := cloudevents.NewClientObserved(p, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())

	if crStatusEventClient == nil {
		crStatusEventClient = crstatusevent.GetDefaultClient()
	}
	if err != nil {
		return nil, err
	}
	return &client{
		ceClient:            ceClient,
		ceOverrides:         ceOverrides,
		reporter:            reporter,
		crStatusEventClient: *crStatusEventClient,
	}, nil
}

func setTimeOut(duration time.Duration) http.Option {
	return func(p *http.Protocol) error {
		if p == nil {
			return fmt.Errorf("http target option can not set nil protocol")
		}
		if p.Client == nil {
			p.Client = &nethttp.Client{}
		}
		p.Client.Timeout = duration
		return nil
	}
}

type client struct {
	ceClient            cloudevents.Client
	ceOverrides         *duckv1.CloudEventOverrides
	reporter            source.StatsReporter
	crStatusEventClient crstatusevent.CRStatusEventClient
}

var _ cloudevents.Client = (*client)(nil)

// Send implements client.Send
func (c *client) Send(ctx context.Context, out event.Event) protocol.Result {
	c.applyOverrides(&out)
	res := c.ceClient.Send(ctx, out)
	return c.reportCount(ctx, out, res)
}

// Request implements client.Request
func (c *client) Request(ctx context.Context, out event.Event) (*event.Event, protocol.Result) {
	c.applyOverrides(&out)
	resp, res := c.ceClient.Request(ctx, out)
	return resp, c.reportCount(ctx, out, res)
}

// StartReceiver implements client.StartReceiver
func (c *client) StartReceiver(ctx context.Context, fn interface{}) error {
	return errors.New("not implemented")
}

func (c *client) applyOverrides(event *cloudevents.Event) {
	if c.ceOverrides != nil && c.ceOverrides.Extensions != nil {
		for n, v := range c.ceOverrides.Extensions {
			event.SetExtension(n, v)
		}
	}
}

func (c *client) reportCount(ctx context.Context, event cloudevents.Event, result protocol.Result) error {
	tags := MetricTagFromContext(ctx)
	reportArgs := &source.ReportArgs{
		Namespace:     tags.Namespace,
		EventSource:   event.Source(),
		EventType:     event.Type(),
		Name:          tags.Name,
		ResourceGroup: tags.ResourceGroup,
	}

	var rres *http.RetriesResult
	if cloudevents.ResultAs(result, &rres) {
		result = rres.Result
	}

	if cloudevents.IsACK(result) {
		var res *http.Result

		if !cloudevents.ResultAs(result, &res) {
			return c.reportError(reportArgs, result)
		}

		_ = c.reporter.ReportEventCount(reportArgs, res.StatusCode)
	} else {
		c.crStatusEventClient.ReportCRStatusEvent(ctx, result)

		var res *http.Result
		if !cloudevents.ResultAs(result, &res) {
			return c.reportError(reportArgs, result)
		}

		if rErr := c.reporter.ReportEventCount(reportArgs, res.StatusCode); rErr != nil {
			// metrics is not important enough to return an error if it is setup wrong.
			// So combine reporter error with ce error if not nil.
			if result != nil {
				result = fmt.Errorf("%w\nmetrics reporter error: %s", result, rErr)
			}
		}
	}
	return result
}

func (c *client) reportError(reportArgs *source.ReportArgs, result protocol.Result) error {
	var uErr *url.Error
	if errors.As(result, &uErr) {
		reportArgs.Timeout = uErr.Timeout()
	}

	if result != nil {
		reportArgs.Error = result.Error()
	}

	if rErr := c.reporter.ReportEventCount(reportArgs, 0); rErr != nil {
		// metrics is not important enough to return an error if it is setup wrong.
		// So combine reporter error with ce error if not nil.
		if result != nil {
			result = fmt.Errorf("%w\nmetrics reporter error: %s", result, rErr)
		}
	}

	return result
}

// Metric context

type MetricTag struct {
	Name          string
	Namespace     string
	ResourceGroup string
}

type metricKey struct{}

// ContextWithMetricTag returns a copy of parent context in which the
// value associated with metric key is the supplied metric tag.
func ContextWithMetricTag(ctx context.Context, metric *MetricTag) context.Context {
	return context.WithValue(ctx, metricKey{}, metric)
}

// MetricTagFromContext returns the metric tag stored in context.
// Returns nil if no metric tag is set in context, or if the stored value is
// not of correct type.
func MetricTagFromContext(ctx context.Context) *MetricTag {
	if metrictag, ok := ctx.Value(metricKey{}).(*MetricTag); ok {
		return metrictag
	}
	return &MetricTag{
		Name:          "unknown",
		Namespace:     "unknown",
		ResourceGroup: "unknown",
	}
}
