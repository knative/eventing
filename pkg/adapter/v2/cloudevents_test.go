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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/adapter/v2/test"
)

type mockReporter struct {
	eventCount int
}

var (
	fakeURL = "test-source"
)

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount += 1
	return nil
}

func TestNewCloudEventsClient_send(t *testing.T) {
	testCases := map[string]struct {
		ceOverrides *duckv1.CloudEventOverrides
		event       *cloudevents.Event
	}{
		"none": {},
		"send": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.type")
				return &event
			}(),
		},
		"send with ceOverrides": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.type")
				return &event
			}(),
			ceOverrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{
				"foo": "bar",
			}},
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
				err := got.Send(context.TODO(), *tc.event)
				if !cloudevents.IsACK(err) {
					t.Fatal(err)
				}
				validateSent(t, innerClient, tc.event.Type())
				validateMetric(t, got.reporter, 1)
			} else {
				validateNotSent(t, innerClient)
			}
		})
	}
}

func TestNewCloudEventsClient_request(t *testing.T) {
	testCases := map[string]struct {
		ceOverrides *duckv1.CloudEventOverrides
		event       *cloudevents.Event
	}{
		"none": {},
		"send": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.type")
				return &event
			}(),
		},
		"send with ceOverrides": {
			event: func() *cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("abc-123")
				event.SetSource("unit/test")
				event.SetType("unit.type")
				return &event
			}(),
			ceOverrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{
				"foo": "bar",
			}},
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
				if !cloudevents.IsACK(err) {
					t.Fatal(err)
				}
				validateSent(t, innerClient, tc.event.Type())
				validateMetric(t, got.reporter, 1)
			} else {
				validateNotSent(t, innerClient)
			}
		})
	}
}

func validateSent(t *testing.T, ce *test.TestCloudEventsClient, want string) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Type(); got != want {
		t.Errorf("Expected %q event to be sent, got %q", want, got)
	}
}

func validateNotSent(t *testing.T, ce *test.TestCloudEventsClient) {
	if got := len(ce.Sent()); got != 0 {
		t.Errorf("Expected 0 event to be sent, got %d", got)
	}
}

func validateMetric(t *testing.T, reporter source.StatsReporter, want int) {
	if mockReporter, ok := reporter.(*mockReporter); !ok {
		t.Errorf("reporter is not a mockReporter")
	} else if mockReporter.eventCount != want {
		t.Errorf("Expected %d for metric, got %d", want, mockReporter.eventCount)
	}
}
