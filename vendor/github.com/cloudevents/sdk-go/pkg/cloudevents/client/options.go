package client

import (
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/nats"
	nethttp "net/http"
	"net/url"
	"strings"
)

type Option func(*ceClient) error

// WithTarget sets the outbound recipient of cloudevents when using an HTTP request.
func WithTarget(targetUrl string) Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			targetUrl = strings.TrimSpace(targetUrl)
			if targetUrl != "" {
				var err error
				var target *url.URL
				target, err = url.Parse(targetUrl)
				if err != nil {
					return fmt.Errorf("client option failed to parse target url: %s", err.Error())
				}

				if t.Req == nil {
					t.Req = &nethttp.Request{
						Method: nethttp.MethodPost,
					}
				}
				t.Req.URL = target
				return nil
			}
			return fmt.Errorf("target option was empty string")
		}
		return fmt.Errorf("invalid target client option received for transport type")
	}
}

// WithHTTPMethod sets the outbound recipient of cloudevents when using an HTTP request.
func WithHTTPMethod(method string) Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			method = strings.TrimSpace(method)
			if method != "" {
				if t.Req == nil {
					t.Req = &nethttp.Request{}
				}
				t.Req.Method = method
				return nil
			}
			return fmt.Errorf("method option was empty string")
		}
		return fmt.Errorf("invalid HTTP method client option received for transport type")
	}
}

// WithHTTPEncoding sets the encoding for clients with HTTP transports.
func WithHTTPEncoding(encoding http.Encoding) Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			t.Encoding = encoding
			return nil
		}
		return fmt.Errorf("invalid HTTP encoding client option received for transport type")
	}
}

// WithHTTPDefaultEncodingSelector sets the encoding selection strategy for
// default encoding selections based on Event.
func WithHTTPDefaultEncodingSelector(fn http.EncodingSelector) Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			if fn != nil {
				t.DefaultEncodingSelectionFn = fn
				return nil
			}
			return fmt.Errorf("fn for DefaultEncodingSelector was nil")
		}
		return fmt.Errorf("invalid HTTP default encoding selector client option received for transport type")
	}
}

// WithHTTPBinaryEncodingSelector sets the encoding selection strategy for
// default encoding selections based on Event, the encoded event will be the
// given version in Binary form.
func WithHTTPBinaryEncoding() Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			t.DefaultEncodingSelectionFn = http.DefaultBinaryEncodingSelectionStrategy
			return nil
		}
		return fmt.Errorf("invalid HTTP binary encoding client option received for transport type")
	}
}

// WithHTTPStructuredEncodingSelector sets the encoding selection strategy for
// default encoding selections based on Event, the encoded event will be the
//// given version in Structured form.
func WithHTTPStructuredEncoding() Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			t.DefaultEncodingSelectionFn = http.DefaultStructuredEncodingSelectionStrategy
			return nil
		}
		return fmt.Errorf("invalid HTTP structured encoding client option received for transport type")
	}
}

// WithHTTPPort sets the port for for clients with HTTP transports.
func WithHTTPPort(port int) Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			if port == 0 {
				return fmt.Errorf("client option was given an invalid port: %d", port)
			}
			t.Port = port
			return nil
		}
		return fmt.Errorf("invalid HTTP port client option received for transport type")
	}
}

// WithHTTPClient sets the internal HTTP client for cloudevent clients with HTTP transports.
func WithHTTPClient(netclient *nethttp.Client) Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*http.Transport); ok {
			if netclient == nil {
				return fmt.Errorf("client option was given an nil HTTP client")
			}
			t.Client = netclient
			return nil
		}
		return fmt.Errorf("invalid HTTP client client option received for transport type")
	}
}

// WithNATSEncoding sets the encoding for clients with NATS transport.
func WithNATSEncoding(encoding nats.Encoding) Option {
	return func(c *ceClient) error {
		if t, ok := c.transport.(*nats.Transport); ok {
			t.Encoding = encoding
			return nil
		}
		return fmt.Errorf("invalid NATS encoding client option received for transport type")
	}
}

// WithEventDefaulter adds an event defaulter to the end of the defaulter chain.
func WithEventDefaulter(fn EventDefaulter) Option {
	return func(c *ceClient) error {
		if fn == nil {
			return fmt.Errorf("client option was given an nil event defaulter")
		}
		c.eventDefaulterFns = append(c.eventDefaulterFns, fn)
		return nil
	}
}

// WithUUIDs adds DefaultIDToUUIDIfNotSet event defaulter to the end of the
// defaulter chain.
func WithUUIDs() Option {
	return func(c *ceClient) error {
		c.eventDefaulterFns = append(c.eventDefaulterFns, DefaultIDToUUIDIfNotSet)
		return nil
	}
}

// WithTimeNow adds DefaultTimeToNowIfNotSet event defaulter to the end of the
// defaulter chain.
func WithTimeNow() Option {
	return func(c *ceClient) error {
		c.eventDefaulterFns = append(c.eventDefaulterFns, DefaultTimeToNowIfNotSet)
		return nil
	}
}
