/*
Copyright 2018 The Knative Authors

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

package channel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	cehttp "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/http"
	"github.com/google/go-cmp/cmp"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/pkg/utils"

	"go.uber.org/zap"
)

func TestEventReceiver_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		method       string
		host         string
		path         string
		header       http.Header
		body         string
		expected     int
		receiverFunc ReceiverFunc
	}{
		"non '/' path": {
			path:     "/something",
			expected: http.StatusNotFound,
		},
		"not a POST": {
			method:   http.MethodGet,
			expected: http.StatusMethodNotAllowed,
		},
		"invalid host name": {
			host:     "no-dot",
			expected: http.StatusInternalServerError,
		},
		"unknown channel error": {
			receiverFunc: func(_ context.Context, c ChannelReference, _ cloudevents.Event) error {
				return &UnknownChannelError{c: c}
			},
			expected: http.StatusNotFound,
		},
		"other receiver function error": {
			receiverFunc: func(_ context.Context, _ ChannelReference, _ cloudevents.Event) error {
				return errors.New("test induced receiver function error")
			},
			expected: http.StatusInternalServerError,
		},
		"headers and body pass through": {
			// The header, body, and host values set here are verified in the receiverFunc. Altering
			// them here will require the same alteration in the receiverFunc.
			header: map[string][]string{
				"not":                       {"passed", "through"},
				"nor":                       {"this-one"},
				"x-requEst-id":              {"1234"},
				"knatIve-will-pass-through": {"true", "always"},
				// Ce headers won't pass through our header filtering as they should actually be set in the CloudEvent itself,
				// as extensions. The SDK then sets them as as Ce- headers when sending them through HTTP.
				"cE-not-pass-through": {"true"},
				"x-B3-pass":           {"will not pass"},
				"x-ot-pass":           {"will not pass"},
			},
			body: "event-body",
			host: "test-name.test-namespace.svc." + utils.GetClusterDomainName(),
			receiverFunc: func(ctx context.Context, r ChannelReference, e cloudevents.Event) error {
				if r.Namespace != "test-namespace" || r.Name != "test-name" {
					return fmt.Errorf("test receiver func -- bad reference: %v", r)
				}
				payload := fmt.Sprintf("%v", e.Data)
				if payload != "event-body" {
					return fmt.Errorf("test receiver func -- bad payload: %v", payload)
				}
				expectedHeaders := map[string]string{
					"x-requEst-id": "1234",
					// Note that only the first value was passed through, the remaining values were
					// discarded.
					"knatIve-will-pass-through": "true",
				}
				tctx := cloudevents.HTTPTransportContextFrom(ctx)
				actualHeaders := make(map[string]string)
				for h, v := range tctx.Header {
					actualHeaders[h] = v[0]
				}
				if diff := cmp.Diff(expectedHeaders, actualHeaders); diff != "" {
					return fmt.Errorf("test receiver func -- bad headers (-want, +got): %s", diff)
				}
				var h string
				if err := e.ExtensionAs(EventHistory, &h); err != nil {
					return fmt.Errorf("test receiver func -- history not added: %v", err)
				}
				expectedHistory := "test-name.test-namespace.svc." + utils.GetClusterDomainName()
				if h != expectedHistory {
					return fmt.Errorf("test receiver func -- bad history: %v", h)
				}
				return nil
			},
			expected: http.StatusAccepted,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			// Default the common things.
			if tc.method == "" {
				tc.method = http.MethodPost
			}
			if tc.path == "" {
				tc.path = "/"
			}
			if tc.host == "" {
				tc.host = "test-channel.test-namespace.svc." + utils.GetClusterDomainName()
			}

			f := tc.receiverFunc
			r, err := NewEventReceiver(f, zap.NewNop())
			if err != nil {
				t.Fatalf("Error creating new event receiver. Error:%s", err)
			}

			ctx := context.Background()
			tctx := cloudevents.HTTPTransportContextFrom(ctx)
			tctx.Host = tc.host
			tctx.Method = tc.method
			tctx.Header = tc.header
			tctx.URI = tc.path
			ctx = cehttp.WithTransportContext(ctx, tctx)

			event := cloudevents.NewEvent(cloudevents.VersionV1)
			event.Data = tc.body
			eventResponse := cloudevents.EventResponse{}

			err = r.ServeHTTP(ctx, event, &eventResponse)
			if eventResponse.Status != tc.expected {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				t.Fatalf("Unexpected status code. Expected %v. Actual %v", tc.expected, eventResponse.Status)
			}
		})
	}
}
func TestEventReceiver_ParseChannel(t *testing.T) {
	c, err := ParseChannel("test-channel.test-namespace.svc.")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if c.Name != "test-channel" {
		t.Errorf("Expected Name: test-channel. Got: %q", c.Name)
	}
	if c.Namespace != "test-namespace" {
		t.Errorf("Expected Name: test-namespace. Got: %q", c.Namespace)
	}
}
