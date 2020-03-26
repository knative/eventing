package adapter

import (
	"context"
	"errors"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/source"
)

// TODO: this could move into the kncloudevents package in the future.

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
		reporter:    reporter,
	}, nil
}

type client struct {
	ceClient    cloudevents.Client
	ceOverrides *duckv1.CloudEventOverrides
	reporter    source.StatsReporter
}

var _ cloudevents.Client = (*client)(nil)

// Send implements client.Send
func (c *client) Send(ctx context.Context, out event.Event) error {
	c.applyOverrides(ctx, &out)
	err := c.ceClient.Send(ctx, out)
	err = c.reportCount(ctx, out, err)

	return err
}

// Request implements client.Request
func (c *client) Request(ctx context.Context, out event.Event) (*event.Event, error) {
	c.applyOverrides(ctx, &out)
	resp, err := c.ceClient.Request(ctx, out)
	err = c.reportCount(ctx, out, err)

	return resp, err
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

func (c *client) reportCount(ctx context.Context, event cloudevents.Event, err error) error {
	tags := MetricTagFromContext(ctx)
	reportArgs := &source.ReportArgs{
		Namespace:     tags.Namespace,
		EventSource:   event.Source(),
		EventType:     event.Type(),
		Name:          tags.Name,
		ResourceGroup: tags.ResourceGroup,
	}
	if rErr := c.reporter.ReportEventCount(reportArgs, statusCode(err)); rErr != nil {
		// metrics is not important enough to return an error if it is setup wrong.
		// So combine reporter error with ce error if not nil.
		if err != nil {
			err = fmt.Errorf("%w\nmetrics reporter errror: %s", err, rErr)
		}
	}
	return err
}

func statusCode(err error) int {
	// TODO: there is talk to change this in the sdk, for now we do not have access to the real http code.
	if err != nil {
		return 400
	}
	return 200
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
	if logger, ok := ctx.Value(metricKey{}).(*MetricTag); ok {
		return logger
	}
	return &MetricTag{
		Name:          "unknown",
		Namespace:     "unknown",
		ResourceGroup: "unknown",
	}
}
