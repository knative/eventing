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

package filter

import (
	"testing"
	"time"

	"knative.dev/eventing/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"
)

// unregister, ehm, unregisters the metrics that were registered, by
// virtue of StatsReporter creation.
// Since golang executes test iterations within the same process, the stats reporter
// returns an error if the metric is already registered and the test panics.
func unregister() {
	metricstest.Unregister("event_count", "dispatch_latencies", "filter_latencies", "event_latencies")
}

func TestStatsReporter(t *testing.T) {
	args := &ReportArgs{
		ns:          "testns",
		trigger:     "testtrigger",
		broker:      "testbroker",
		eventType:   "testeventtype",
		eventSource: "testeventsource",
	}

	r, err := NewStatsReporter()
	if err != nil {
		t.Fatalf("Failed to create a new reporter: %v", err)
	}
	// Without this `go test ... -count=X`, where X > 1, fails, since
	// we get an error about view already being registered.
	defer unregister()

	wantTags := map[string]string{
		metricskey.NamespaceName: "testns",
		metricskey.TriggerName:   "testtrigger",
		metricskey.BrokerName:    "testbroker",
		metricskey.TriggerType:   "testeventtype",
		metricskey.TriggerSource: "testeventsource",
	}

	wantTags1 := map[string]string(wantTags)
	wantTags1[metricskey.Result] = "success"

	// test ReportEventCount
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, nil)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, nil)
	})
	metricstest.CheckCountData(t, "event_count", wantTags1, 2)

	// test ReportDispatchTime
	expectSuccess(t, func() error {
		return r.ReportDispatchTime(args, nil, 1100*time.Millisecond)
	})
	expectSuccess(t, func() error {
		return r.ReportDispatchTime(args, nil, 9100*time.Millisecond)
	})
	metricstest.CheckDistributionData(t, "dispatch_latencies", wantTags1, 2, 1100.0, 9100.0)

	// test ReportEventDeliveryTime
	expectSuccess(t, func() error {
		return r.ReportEventDeliveryTime(args, nil, 1000*time.Millisecond)
	})
	expectSuccess(t, func() error {
		return r.ReportEventDeliveryTime(args, nil, 8000*time.Millisecond)
	})
	metricstest.CheckDistributionData(t, "event_latencies", wantTags1, 2, 1000.0, 8000.0)

	wantTags2 := map[string]string(wantTags)
	wantTags2[metricskey.FilterResult] = "pass"

	// test ReportFilterTime
	expectSuccess(t, func() error {
		return r.ReportFilterTime(args, "pass", 100*time.Millisecond)
	})
	expectSuccess(t, func() error {
		return r.ReportFilterTime(args, "pass", 500*time.Millisecond)
	})
	metricstest.CheckDistributionData(t, "filter_latencies", wantTags1, 2, 100.0, 500.0)
}

func TestReporterEmptySourceAndType(t *testing.T) {
	r, err := NewStatsReporter()
	defer unregister()

	if err != nil {
		t.Fatalf("Failed to create a new reporter: %v", err)
	}

	args := &ReportArgs{
		ns:          "testns",
		trigger:     "testtrigger",
		broker:      "testbroker",
		eventType:   "",
		eventSource: "",
	}

	wantTags := map[string]string{
		metricskey.NamespaceName: "testns",
		metricskey.TriggerName:   "testtrigger",
		metricskey.BrokerName:    "testbroker",
		metricskey.TriggerType:   metricskey.Any,
		metricskey.TriggerSource: metricskey.Any,
		metricskey.Result:        "success",
	}

	// test ReportEventCount
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, nil)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, nil)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, nil)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, nil)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 4)
}

func expectSuccess(t *testing.T, f func() error) {
	t.Helper()
	if err := f(); err != nil {
		t.Errorf("Reporter expected success but got error: %v", err)
	}
}
