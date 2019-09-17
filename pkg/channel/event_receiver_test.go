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
	"github.com/cloudevents/sdk-go"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"knative.dev/eventing/pkg/utils"
	_ "knative.dev/pkg/system/testing"

	"go.uber.org/zap"
)

func TestMessageReceiver_HandleRequest(t *testing.T) {
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
			receiverFunc: func(_ context.Context, _ ChannelReference, _ cloudevents.Event) error {
				return ErrUnknownChannel
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
				"contenT-type":              {"text/json"},
				"knatIve-will-pass-through": {"true", "always"},
				"cE-pass-through":           {"true"},
				"x-B3-pass":                 {"true"},
				"x-ot-pass":                 {"true"},
			},
			body: "message-body",
			host: "test-name.test-namespace.svc." + utils.GetClusterDomainName(),
			receiverFunc: func(ctx context.Context, r ChannelReference, e cloudevents.Event) error {
				if r.Namespace != "test-namespace" || r.Name != "test-name" {
					return fmt.Errorf("test receiver func -- bad reference: %v", r)
				}
				payload := fmt.Sprintf("%v", e.Data)
				if payload != "message-body" {
					return fmt.Errorf("test receiver func -- bad payload: %v", payload)
				}
				expectedHeaders := map[string]string{
					"x-requEst-id": "1234",
					"contenT-type": "text/json",
					// Note that only the first value was passed through, the remaining values were
					// discarded.
					"knatIve-will-pass-through": "true",
					"cE-pass-through":           "true",
					"x-B3-pass":                 "true",
					"x-ot-pass":                 "true",
					"ce-knativehistory":         "test-name.test-namespace.svc." + utils.GetClusterDomainName(),
				}
				tctx := cloudevents.HTTPTransportContextFrom(ctx)
				actualHeaders := make(map[string]string)
				for h, v := range tctx.Header {
					actualHeaders[h] = v[0]
				}
				if diff := cmp.Diff(expectedHeaders, actualHeaders); diff != "" {
					return fmt.Errorf("test receiver func -- bad headers (-want, +got): %s", diff)
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
				t.Fatalf("Error creating new message receiver. Error:%s", err)
			}

			ctx := context.Background()
			tctx := cloudevents.HTTPTransportContextFrom(ctx)
			tctx.Host = tc.host
			tctx.Method = tc.method
			tctx.Header = tc.header
			tctx.URI = tc.path
			sctx := utils.SendingContext(ctx, tctx, nil)

			event := cloudevents.NewEvent(cloudevents.VersionV03)
			event.Data = tc.body
			eventResponse := cloudevents.EventResponse{}

			r.serveHTTP(sctx, event, &eventResponse)
			if eventResponse.Status != tc.expected {
				t.Fatalf("Unexpected status code. Expected %v. Actual %v", tc.expected, eventResponse.Status)
			}
		})
	}
}
