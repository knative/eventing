package kncloudevents

import (
	nethttp "net/http"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"knative.dev/pkg/tracing"
)

// ConnectionArgs allow to configure connection parameters to the underlying
// HTTP Client transport.
type ConnectionArgs struct {
	// MaxIdleConns refers to the max idle connections, as in net/http/transport.
	MaxIdleConns int
	// MaxIdleConnsPerHost refers to the max idle connections per host, as in net/http/transport.
	MaxIdleConnsPerHost int
}

func NewDefaultClient(target ...string) (cloudevents.Client, error) {
	tOpts := []http.Option{
		cloudevents.WithBinaryEncoding(),
		// Add input tracing.
		http.WithMiddleware(tracing.HTTPSpanMiddleware),
	}
	if len(target) > 0 && target[0] != "" {
		tOpts = append(tOpts, cloudevents.WithTarget(target[0]))
	}

	// Make an http transport for the CloudEvents client.
	t, err := cloudevents.NewHTTPTransport(tOpts...)
	if err != nil {
		return nil, err
	}
	return NewDefaultClientGivenHttpTransport(t)
}

// NewDefaultClientGivenHttpTransport creates a new CloudEvents client using the provided cloudevents HTTP
// transport. Note that it does modify the provided cloudevents HTTP Transport by adding tracing to its Client
// and different connection options, in case they are specified.
func NewDefaultClientGivenHttpTransport(t *cloudevents.HTTPTransport, connectionArgs ...ConnectionArgs) (cloudevents.Client, error) {
	// Add connection options to the default transport.
	var base = nethttp.DefaultTransport
	if len(connectionArgs) > 0 {
		baseTransport := base.(*nethttp.Transport)
		baseTransport.MaxIdleConns = connectionArgs[0].MaxIdleConns
		baseTransport.MaxIdleConnsPerHost = connectionArgs[0].MaxIdleConnsPerHost
	}
	// Add output tracing.
	t.Client = &nethttp.Client{
		Transport: &ochttp.Transport{
			Base:        base,
			Propagation: &b3.HTTPFormat{},
		},
	}

	// Use the transport to make a new CloudEvents client.
	c, err := cloudevents.NewClient(t,
		cloudevents.WithUUIDs(),
		cloudevents.WithTimeNow(),
	)

	if err != nil {
		return nil, err
	}
	return c, nil
}
