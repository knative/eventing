/*
Copyright 2019 The Knative Authors

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

package apiserver

import (
	"net/http"
	"testing"

	. "knative.dev/eventing/pkg/metrics/metricskey"
	metricskeyEventing "knative.dev/eventing/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"
)

// unregister, ehm, unregisters the metrics that were registered, by
// virtue of StatsReporter creation.
// Since golang executes test iterations within the same process, the stats reporter
// returns an error if the metric is already registered and the test panics.
func unregister() {
	metricstest.Unregister("event_count")
}

func TestStatsReporter(t *testing.T) {
	args := &ReportArgs{
		ns:          "testns",
		eventType:   "dev.knative.apiserver.resource.update",
		eventSource: "unit-test",
		name:        "testimporter",
	}

	r, err := NewStatsReporter()
	if err != nil {
		t.Fatalf("Failed to create a new reporter: %v", err)
	}
	// Without this `go test ... -count=X`, where X > 1, fails, since
	// we get an error about view already being registered.
	defer unregister()

	wantTags := map[string]string{
		metricskey.LabelNamespaceName:         "testns",
		metricskey.LabelEventType:             "dev.knative.apiserver.resource.update",
		metricskey.LabelEventSource:           "unit-test",
		metricskey.LabelImporterName:          "testimporter",
		metricskey.LabelImporterResourceGroup: "apiserversources.sources.eventing.knative.dev",
		LabelResponseCode:                     "202",
		LabelResponseCodeClass:                "2xx",
	}

	// test ReportEventCount
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusAccepted)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusAccepted)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 2)
}

func TestReporterFor5xxResponse(t *testing.T) {
	r, err := NewStatsReporter()
	defer unregister()

	if err != nil {
		t.Fatalf("Failed to create a new reporter: %v", err)
	}

	args := &ReportArgs{
		ns:          "testns",
		eventType:   "eventtype",
		eventSource: "eventsource",
		name:        "testimporter",
	}

	wantTags := map[string]string{
		metricskey.LabelNamespaceName:         "testns",
		metricskeyEventing.LabelFilterResult:  "success",
		metricskey.LabelEventType:             "eventtype",
		metricskey.LabelEventSource:           "eventsource",
		metricskey.LabelImporterName:          "testimporter",
		metricskey.LabelImporterResourceGroup: "apiserversources.sources.eventing.knative.dev",
		LabelResponseCode:                     "500",
		LabelResponseCodeClass:                "5xx",
	}
	// test ReportEventCount
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusInternalServerError)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusInternalServerError)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 2)
}

func expectSuccess(t *testing.T, f func() error) {
	t.Helper()
	if err := f(); err != nil {
		t.Errorf("Reporter expected success but got error: %v", err)
	}
}
