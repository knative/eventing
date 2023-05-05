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

	obshttp "github.com/cloudevents/sdk-go/observability/opencensus/v2/http"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/plugin/ochttp"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/metrics/source"
	obsclient "knative.dev/eventing/pkg/observability/client"
)

type closeIdler interface {
	CloseIdleConnections()
}

type Client interface {
	cloudevents.Client
	closeIdler
}

var newClientHTTPObserved = NewClientHTTPObserved

func NewClientHTTPObserved(topt []http.Option, copt []ceclient.Option) (Client, error) {
	t, err := obshttp.NewObservedHTTP(topt...)
	if err != nil {
		return nil, err
	}

	copt = append(copt, ceclient.WithTimeNow(), ceclient.WithUUIDs(), ceclient.WithObservabilityService(obsclient.New()))

	c, err := ceclient.New(t, copt...)
	if err != nil {
		return nil, err
	}

	return &client{
		ceClient: c,
	}, nil
}

// NewCloudEventsClient returns a client that will apply the ceOverrides to
// outbound events and report outbound event counts.
func NewCloudEventsClient(target string, ceOverrides *duckv1.CloudEventOverrides, reporter source.StatsReporter) (Client, error) {
	opts := make([]http.Option, 0)
	if len(target) > 0 {
		opts = append(opts, cloudevents.WithTarget(target))
	}
	return NewClient(ClientConfig{
		CeOverrides: ceOverrides,
		Reporter:    reporter,
		Options:     opts,
	})
}

// NewCloudEventsClientWithOptions returns a client created with provided options
func NewCloudEventsClientWithOptions(ceOverrides *duckv1.CloudEventOverrides, reporter source.StatsReporter, opts ...http.Option) (Client, error) {
	return NewClient(ClientConfig{
		CeOverrides: ceOverrides,
		Reporter:    reporter,
		Options:     opts,
	})
}

// NewCloudEventsClientCRStatus returns a client CR status
func NewCloudEventsClientCRStatus(env EnvConfigAccessor, reporter source.StatsReporter, crStatusEventClient *crstatusevent.CRStatusEventClient) (Client, error) {
	return NewClient(ClientConfig{
		Env:                 env,
		Reporter:            reporter,
		CrStatusEventClient: crStatusEventClient,
	})
}

type ClientConfig struct {
	Env                 EnvConfigAccessor
	CeOverrides         *duckv1.CloudEventOverrides
	Reporter            source.StatsReporter
	CrStatusEventClient *crstatusevent.CRStatusEventClient
	Options             []http.Option

	Client Client
}

type clientConfigKey struct{}

func withClientConfig(ctx context.Context, r ClientConfig) context.Context {
	return context.WithValue(ctx, clientConfigKey{}, r)
}

func GetClientConfig(ctx context.Context) ClientConfig {
	val := ctx.Value(clientConfigKey{})
	if val == nil {
		return ClientConfig{}
	}
	return val.(ClientConfig)
}

func NewClient(cfg ClientConfig) (Client, error) {
	if cfg.Client != nil {
		return cfg.Client, nil
	}

	transport := &ochttp.Transport{
		Propagation: tracecontextb3.TraceContextEgress,
	}

	pOpts := make([]http.Option, 0)
	var closeIdler closeIdler = nethttp.DefaultTransport.(*nethttp.Transport)

	ceOverrides := cfg.CeOverrides
	if cfg.Env != nil {
		if target := cfg.Env.GetSink(); len(target) > 0 {
			pOpts = append(pOpts, cloudevents.WithTarget(target))
		}
		if sinkWait := cfg.Env.GetSinktimeout(); sinkWait > 0 {
			pOpts = append(pOpts, setTimeOut(time.Duration(sinkWait)*time.Second))
		}
		if eventingtls.IsHttpsSink(cfg.Env.GetSink()) {
			var err error

			clientConfig := eventingtls.NewDefaultClientConfig()
			clientConfig.CACerts = cfg.Env.GetCACerts()

			httpTransport := nethttp.DefaultTransport.(*nethttp.Transport).Clone()
			httpTransport.TLSClientConfig, err = eventingtls.GetTLSClientConfig(clientConfig)
			if err != nil {
				return nil, err
			}

			closeIdler = httpTransport

			transport = &ochttp.Transport{
				Base:        httpTransport,
				Propagation: tracecontextb3.TraceContextEgress,
			}
		}
		if ceOverrides == nil {
			var err error
			ceOverrides, err = cfg.Env.GetCloudEventOverrides()
			if err != nil {
				return nil, err
			}
		}
	}

	pOpts = append(pOpts, http.WithRoundTripper(transport))

	// Make sure that explicitly set options have priority
	opts := append(pOpts, cfg.Options...)

	ceClient, err := newClientHTTPObserved(opts, nil)

	if cfg.CrStatusEventClient == nil {
		cfg.CrStatusEventClient = crstatusevent.GetDefaultClient()
	}
	if err != nil {
		return nil, err
	}
	return &client{
		ceClient:            ceClient,
		closeIdler:          closeIdler,
		ceOverrides:         ceOverrides,
		reporter:            cfg.Reporter,
		crStatusEventClient: cfg.CrStatusEventClient,
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
	crStatusEventClient *crstatusevent.CRStatusEventClient
	closeIdler          closeIdler
}

func (c *client) CloseIdleConnections() {
	c.closeIdler.CloseIdleConnections()
}

var _ cloudevents.Client = (*client)(nil)

// Send implements client.Send
func (c *client) Send(ctx context.Context, out event.Event) protocol.Result {
	c.applyOverrides(&out)
	res := c.ceClient.Send(ctx, out)
	c.reportMetrics(ctx, out, res)
	return res
}

// Request implements client.Request
func (c *client) Request(ctx context.Context, out event.Event) (*event.Event, protocol.Result) {
	c.applyOverrides(&out)
	resp, res := c.ceClient.Request(ctx, out)
	c.reportMetrics(ctx, out, res)
	return resp, res
}

// StartReceiver implements client.StartReceiver
func (c *client) StartReceiver(ctx context.Context, fn interface{}) error {
	return c.ceClient.StartReceiver(ctx, fn)
}

func (c *client) applyOverrides(event *cloudevents.Event) {
	if c.ceOverrides != nil && c.ceOverrides.Extensions != nil {
		for n, v := range c.ceOverrides.Extensions {
			event.SetExtension(n, v)
		}
	}
}

func (c *client) reportMetrics(ctx context.Context, event cloudevents.Event, result protocol.Result) {
	if c.reporter == nil {
		return
	}
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
			c.reportError(reportArgs, result)
		}

		_ = c.reporter.ReportEventCount(reportArgs, res.StatusCode)
	} else {
		c.crStatusEventClient.ReportCRStatusEvent(ctx, result)

		var res *http.Result
		if !cloudevents.ResultAs(result, &res) {
			c.reportError(reportArgs, result)
		} else {
			c.reporter.ReportEventCount(reportArgs, res.StatusCode)
		}
	}
	if rres != nil && len(rres.Attempts) > 0 {
		for _, retryResult := range rres.Attempts {
			var res *http.Result
			if !cloudevents.ResultAs(retryResult, &res) {
				c.reportError(reportArgs, result)
			} else {
				c.reporter.ReportRetryEventCount(reportArgs, res.StatusCode)
			}
		}
	}
}

func (c *client) reportError(reportArgs *source.ReportArgs, result protocol.Result) {
	var uErr *url.Error
	if errors.As(result, &uErr) {
		reportArgs.Timeout = uErr.Timeout()
	}

	if result != nil {
		reportArgs.Error = result.Error()
	}
	c.reporter.ReportEventCount(reportArgs, 0)
}

// MetricTag context
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
