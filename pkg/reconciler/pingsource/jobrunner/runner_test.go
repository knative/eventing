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

package jobrunner

import (
	"context"
	"testing"
	"time"

	"knative.dev/pkg/source"

	kncetesting "knative.dev/eventing/pkg/kncloudevents/testing"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestAddRunRemoveSchedule(t *testing.T) {
	logger := logtesting.TestLogger(t)
	reporter := &mockReporter{}
	ce := kncetesting.NewTestClient()

	runner := NewCronJobsRunner(ce, reporter, logger)

	entryId, err := runner.AddSchedule("test-ns", "test-name", "* * * * ?", "some data", "a sink")

	if err != nil {
		t.Errorf("Should not throw error %v", err)
	}

	entry := runner.cron.Entry(entryId)
	if entry.ID != entryId {
		t.Error("Entry has not been added")
	}

	entry.Job.Run()

	validateSent(t, ce, `{"body":"some data"}`)
	validateMetric(t, reporter, 1)

	runner.RemoveSchedule(entryId)

	entry = runner.cron.Entry(entryId)
	if entry.ID == entryId {
		t.Error("Entry has not been removed")
	}
}

func TestStartStopCron(t *testing.T) {
	logger := logtesting.TestLogger(t)
	reporter := &mockReporter{}
	ce := kncetesting.NewTestClient()

	runner := NewCronJobsRunner(ce, reporter, logger)

	ctx, cancel := context.WithCancel(context.Background())
	wctx, wcancel := context.WithCancel(context.Background())

	go func() {
		err := runner.Start(ctx.Done())
		if err != nil {
			t.Errorf("Cron job runner couldn't start %v", err)
		}
		wcancel()
	}()

	cancel()

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("expected cron to be stopped after 2 seconds")
	case <-wctx.Done():
	}

}

func validateMetric(t *testing.T, reporter source.StatsReporter, want int) {
	if mockReporter, ok := reporter.(*mockReporter); !ok {
		t.Errorf("reporter is not a mockReporter")
	} else if mockReporter.eventCount != want {
		t.Errorf("Expected %d for metric, got %d", want, mockReporter.eventCount)
	}
}

func validateSent(t *testing.T, ce *kncetesting.TestCloudEventsClient, wantData string) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Data; string(got.([]byte)) != wantData {
		t.Errorf("Expected %q event to be sent, got %q", wantData, string(got.([]byte)))
	}
}
