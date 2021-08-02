/*
Copyright 2021 The Knative Authors

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

package source

import (
	"net/http"
	"testing"

	"knative.dev/eventing/pkg/metrics"
	"knative.dev/pkg/metrics/metricstest"
	_ "knative.dev/pkg/metrics/testing"
)

func TestStatsReporter(t *testing.T) {
	setup()

	args := &ReportArgs{
		Namespace:     "testns",
		EventType:     "dev.knative.event",
		EventSource:   "unit-test",
		Name:          "testsource",
		ResourceGroup: "testresourcegroup",
	}

	r, err := NewStatsReporter()
	if err != nil {
		t.Fatal("Failed to create a new reporter:", err)
	}

	wantTags := map[string]string{
		metrics.LabelNamespaceName:     "testns",
		metrics.LabelEventType:         "dev.knative.event",
		metrics.LabelEventSource:       "unit-test",
		metrics.LabelName:              "testsource",
		metrics.LabelResourceGroup:     "testresourcegroup",
		metrics.LabelResponseCode:      "202",
		metrics.LabelResponseCodeClass: "2xx",
	}

	retryWantTags := map[string]string{
		metrics.LabelNamespaceName:     "testns",
		metrics.LabelEventType:         "dev.knative.event",
		metrics.LabelEventSource:       "unit-test",
		metrics.LabelName:              "testsource",
		metrics.LabelResourceGroup:     "testresourcegroup",
		metrics.LabelResponseCode:      "503",
		metrics.LabelResponseCodeClass: "5xx",
	}

	// test ReportEventCount and ReportRetryEventCount
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusAccepted)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusAccepted)
	})
	expectSuccess(t, func() error {
		return r.ReportRetryEventCount(args, http.StatusServiceUnavailable)
	})
	expectSuccess(t, func() error {
		return r.ReportRetryEventCount(args, http.StatusServiceUnavailable)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 2)
	metricstest.CheckCountData(t, "retry_event_count", retryWantTags, 2)
}

func TestBadValues(t *testing.T) {
	r, err := NewStatsReporter()
	if err != nil {
		t.Fatal("Failed to create a new reporter:", err)
	}

	args := &ReportArgs{
		Namespace: "😀",
	}

	if err := r.ReportEventCount(args, 200); err == nil {
		t.Errorf("expected ReportEventCount to return an error")
	}

	if err := r.ReportRetryEventCount(args, 200); err == nil {
		t.Errorf("expected ReportRetryEventCount to return an error")
	}
}

func expectSuccess(t *testing.T, f func() error) {
	t.Helper()
	if err := f(); err != nil {
		t.Error("Reporter expected success but got error:", err)
	}
}

func setup() {
	resetMetrics()
}

func resetMetrics() {
	// OpenCensus metrics carry global state that need to be reset between unit tests.
	metricstest.Unregister("event_count")
	metricstest.Unregister("retry_event_count")
	register()
}
