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
	nethttp "net/http"
	"os"
	"strconv"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	v2client "github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	. "knative.dev/pkg/reconciler/testing"

	"knative.dev/eventing/pkg/adapter/v2/test"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/pkg/metrics/source"
)

type mockReporter struct {
	eventCount      int
	retryEventCount int
}

var (
	fakeURL = "test-source"
)

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount++
	return nil
}

func (r *mockReporter) ReportRetryEventCount(args *source.ReportArgs, responseCode int) error {
	r.retryEventCount++
	return nil
}

func TestNewCloudEventsClient_send(t *testing.T) {
	demoEvent := func() *cloudevents.Event {
		event := cloudevents.NewEvent()
		event.SetID("abc-123")
		event.SetSource("unit/test")
		event.SetType("unit.type")
		return &event
	}
	testCases := map[string]struct {
		ceOverrides    *duckv1.CloudEventOverrides
		event          *cloudevents.Event
		timeout        int
		wantErr        bool
		wantRetryCount bool
		wantEventCount int
	}{
		"timeout": {timeout: 13},
		"none":    {},
		"send": {
			event:          demoEvent(),
			wantEventCount: 1,
		},
		"send with retries": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.retries")
				return &event
			}(),
			wantRetryCount: true,
			wantEventCount: 1,
		},
		"send with ceOverrides": {
			event: demoEvent(),
			ceOverrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{
				"foo": "bar",
			}},
			wantEventCount: 1,
		},
		"send fails": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.sendFail")
				return &event
			}(),
			wantErr:        true,
			wantEventCount: 1,
		},
		"not a http result": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.wantErr")
				return &event
			}(),
			wantErr:        true,
			wantEventCount: 1,
		},
		"retry that is not a http result": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.nonHttpRetry")
				return &event
			}(),
			wantErr:        false,
			wantEventCount: 2,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			restoreHTTP := newClientHTTPObserved
			restoreEnv, setEnv := os.LookupEnv("K_SINK_TIMEOUT")
			if tc.timeout != 0 {
				if err := os.Setenv("K_SINK_TIMEOUT", strconv.Itoa(tc.timeout)); err != nil {
					t.Error(err)
				}
			}

			defer func(restoreHTTP func(topt []http.Option, copt []v2client.Option) (Client, error), restoreEnv string, setEnv bool) {
				newClientHTTPObserved = restoreHTTP
				if setEnv {
					if err := os.Setenv("K_SINK_TIMEOUT", restoreEnv); err != nil {
						t.Error(err)
					}
				} else {
					if err := os.Unsetenv("K_SINK_TIMEOUT"); err != nil {
						t.Error(err)
					}
				}

			}(restoreHTTP, restoreEnv, setEnv)

			sendOptions := []http.Option{}
			newClientHTTPObserved = func(topt []http.Option, copt []v2client.Option) (Client, error) {
				sendOptions = append(sendOptions, topt...)
				return nil, nil
			}
			envConfigAccessor := ConstructEnvOrDie(func() EnvConfigAccessor {
				return &EnvConfig{}

			})

			ceClient, err := NewCloudEventsClientCRStatus(envConfigAccessor, &mockReporter{}, nil)
			if err != nil {
				t.Fail()
			}
			timeoutSet := false
			if tc.timeout != 0 {
				for _, opt := range sendOptions {
					p := http.Protocol{}
					if err := opt(&p); err != nil {
						t.Error(err)
					}
					if p.Client != nil {
						if p.Client.Timeout == time.Duration(tc.timeout)*time.Second {
							timeoutSet = true
						}
					}

				}
				if !timeoutSet {
					t.Error("Expected timeout to be set")
				}

			}
			got, ok := ceClient.(*client)
			if !ok {
				t.Errorf("expected NewCloudEventsClient to return a *client, but did not")
			}
			innerClient := &test.TestCloudEventsClient{}
			got.ceClient = innerClient

			if tc.event != nil {
				err := got.Send(context.TODO(), *tc.event)
				if !tc.wantErr && cloudevents.IsUndelivered(err) {
					t.Fatal(err)
				} else if tc.wantErr && cloudevents.IsACK(err) {
					//handle err
					t.Fatal(err)
				}
				validateSent(t, innerClient, tc.event.Type())
				validateMetric(t, got.reporter, tc.wantEventCount, tc.wantRetryCount)
			} else {
				validateNotSent(t, innerClient)
			}
		})
	}
}

func TestNewCloudEventsClient_request(t *testing.T) {
	demoEvent := func() *cloudevents.Event {
		event := cloudevents.NewEvent()
		event.SetID("abc-123")
		event.SetSource("unit/test")
		event.SetType("unit.type")
		return &event
	}
	testCases := map[string]struct {
		ceOverrides    *duckv1.CloudEventOverrides
		event          *cloudevents.Event
		timeout        int
		wantErr        bool
		wantRetryCount bool
	}{
		"timeout": {timeout: 13},
		"none":    {},
		"send": {
			event: demoEvent(),
		},
		"send with retries": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.retries")
				return &event
			}(),
			wantRetryCount: true,
		},
		"send with ceOverrides": {
			event: demoEvent(),
			ceOverrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{
				"foo": "bar",
			}},
		},
		"send fails": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.sendFail")
				return &event
			}(),
			wantErr: true,
		},
		"not a http result": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.wantErr")
				return &event
			}(),
			wantErr: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ceClient, err := NewCloudEventsClient(fakeURL, tc.ceOverrides, &mockReporter{})
			if err != nil {
				t.Fail()
			}
			got, ok := ceClient.(*client)
			if !ok {
				t.Errorf("expected NewCloudEventsClient to return a *client, but did not")
			}
			innerClient := &test.TestCloudEventsClient{}
			got.ceClient = innerClient

			if tc.event != nil {
				_, err := got.Request(context.TODO(), *tc.event)
				if !tc.wantErr && cloudevents.IsUndelivered(err) {
					t.Fatal(err)
				} else if tc.wantErr && cloudevents.IsACK(err) {
					//handle err
					t.Fatal(err)
				}
				validateSent(t, innerClient, tc.event.Type())
				validateMetric(t, got.reporter, 1, tc.wantRetryCount)
			} else {
				validateNotSent(t, innerClient)
			}
		})
	}
}

func TestTLS(t *testing.T) {
	t.Parallel()

	ctx, _ := SetupFakeContext(t)
	ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, 20*time.Millisecond, 5)
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	ca := eventingtlstesting.StartServer(ctx, t, 8333, nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		if request.TLS == nil {
			// It's not on TLS, fail request
			writer.WriteHeader(nethttp.StatusInternalServerError)
			return
		}
		writer.WriteHeader(nethttp.StatusOK)
	}))

	event := cetest.MinEvent()

	tt := []struct {
		name    string
		sink    string
		caCerts *string
		wantErr bool
	}{
		{
			name:    "https sink URL, no CA certs fail",
			sink:    "https://localhost:8333",
			wantErr: true,
		},
		{
			name:    "https sink URL with ca certs",
			sink:    "https://localhost:8333",
			caCerts: pointer.String(ca),
		},
		{
			name:    "http sink URL with ca certs",
			sink:    "http://localhost:8333",
			caCerts: pointer.String(ca),
			wantErr: true,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reporter, err := source.NewStatsReporter()
			assert.Nil(t, err)

			c, err := NewCloudEventsClientCRStatus(
				&EnvConfig{
					Sink:    tc.sink,
					CACerts: tc.caCerts,
				},
				reporter,
				nil,
			)
			assert.Nil(t, err)

			result := c.Send(ctx, event)
			if tc.wantErr {
				if cloudevents.IsACK(result) {
					t.Fatalf("wantErr %v, got %v IsACK %v", tc.wantErr, result, cloudevents.IsACK(result))
				}
			} else if cloudevents.IsNACK(result) || cloudevents.IsUndelivered(result) {
				t.Fatalf("wantErr %v, got %v IsACK %v", tc.wantErr, result, cloudevents.IsACK(result))
			}

			c.CloseIdleConnections()
		})
	}
}

func validateSent(t *testing.T, ce *test.TestCloudEventsClient, want string) {
	if got := len(ce.Sent()); got != 1 {
		t.Error("Expected 1 event to be sent, got", got)
	}

	if got := ce.Sent()[0].Type(); got != want {
		t.Errorf("Expected %q event to be sent, got %q", want, got)
	}
}

func validateNotSent(t *testing.T, ce *test.TestCloudEventsClient) {
	if got := len(ce.Sent()); got != 0 {
		t.Error("Expected no events to be sent, got", got)
	}
}

func validateMetric(t *testing.T, reporter source.StatsReporter, want int, wantRetryCount bool) {
	if mockReporter, ok := reporter.(*mockReporter); !ok {
		t.Errorf("Reporter is not a mockReporter")
	} else if mockReporter.eventCount != want {
		t.Errorf("Expected %d for metric, got %d", want, mockReporter.eventCount)
	} else if mockReporter.retryEventCount != want && wantRetryCount {
		t.Errorf("Expected %d for metric, got %d", want, mockReporter.retryEventCount)
	}
}
