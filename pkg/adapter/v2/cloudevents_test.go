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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	nethttp "net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/adapter/v2/test"
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
			restoreHTTP := cloudevents.NewHTTP
			restoreEnv, setEnv := os.LookupEnv("K_SINK_TIMEOUT")
			if tc.timeout != 0 {
				if err := os.Setenv("K_SINK_TIMEOUT", strconv.Itoa(tc.timeout)); err != nil {
					t.Error(err)
				}
			}

			defer func(restoreHTTP func(opts ...http.Option) (*http.Protocol, error), restoreEnv string, setEnv bool) {
				cloudevents.NewHTTP = restoreHTTP
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
			cloudevents.NewHTTP = func(opts ...http.Option) (*http.Protocol, error) {
				sendOptions = append(sendOptions, opts...)
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

func TestNewCloudEventsClient_receiver(t *testing.T) {
	sampleEvent := func() *cloudevents.Event {
		event := cloudevents.NewEvent(cloudevents.VersionV1)
		event.SetID("abc-123")
		event.SetSource("unit/test")
		event.SetType("unit.type")
		event.SetDataContentType("application/json")
		return &event
	}

	testCases := map[string]struct {
		Headers      nethttp.Header
		Body         []byte
		ExpectedBody string
		WantTimeout  bool
	}{
		"binary event": {
			Headers: map[string][]string{
				"content-type":   {"application/json"},
				"ce-specversion": {"1.0"},
				"ce-id":          {"abc-123"},
				"ce-type":        {"unit.type"},
				"ce-source":      {"unit/test"},
			},
			Body:         []byte(`{"type": "binary"}`),
			ExpectedBody: `{"type": "binary"}`,
		},
		"structured event": {
			Headers: map[string][]string{
				"content-type": {"application/cloudevents+json"},
			},
			Body: []byte(`{
				"specversion": "1.0",
				"id": "abc-123",
				"source": "unit/test",
				"type": "unit.type",
				"datacontenttype": "application/json",
				"data": {"type": "structured"}
			}`),
			ExpectedBody: ` {"type": "structured"}`,
		},
		"malformed event": {
			Headers: map[string][]string{
				"ce-type": {"err.event"},
			},
			WantTimeout: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			port := rand.Intn(16383) + 49152 // IANA Dynamic Ports range
			ceClient, err := NewCloudEventsClientWithOptions(nil, &mockReporter{}, cloudevents.WithPort(port))
			if err != nil {
				t.Errorf("failed to create CE client: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			events := make(chan event.Event, 1)
			defer close(events)
			errors := make(chan error, 1)
			defer close(errors)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				wg.Done()
				err := ceClient.StartReceiver(ctx, func(event event.Event) error {
					events <- event
					return nil
				})
				if err != nil {
					errors <- err
				}
			}()
			wg.Wait()                         // wait for the above goroutine to be running
			time.Sleep(50 * time.Millisecond) // wait for the receiver to start

			target, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
			if err != nil {
				t.Errorf("failed to parse target URL: %v", err)
			}

			req := &nethttp.Request{
				Method:        "POST",
				URL:           target,
				Header:        tc.Headers,
				Body:          ioutil.NopCloser(bytes.NewReader(tc.Body)),
				ContentLength: int64(len(tc.Body)),
			}

			_, err = nethttp.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("failed to execute the request %s", err.Error())
			}

			select {
			case <-ctx.Done():
				if !tc.WantTimeout {
					t.Errorf("unexpected receiver timeout")
				}
			case got := <-events:
				if tc.WantTimeout {
					t.Errorf("unexpected event received")
				}
				if diff := cmp.Diff(sampleEvent().Context, got.Context); diff != "" {
					t.Errorf("unexpected events.Context (-want, +got) = %v", diff)
				}
				if diff := cmp.Diff(tc.ExpectedBody, string(got.Data())); diff != "" {
					t.Errorf("unexpected events.Data (-want, +got) = %v", diff)
				}
			case err := <-errors:
				t.Errorf("failed to start receiver: %v", err)
			}
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
