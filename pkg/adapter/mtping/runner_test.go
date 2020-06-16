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

package mtping

import (
	"context"
	"reflect"
	"testing"
	"time"

	adaptertesting "knative.dev/eventing/pkg/adapter/v2/test"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestAddRunRemoveSchedules(t *testing.T) {

	testCases := map[string]struct {
		wantCEOverrides *duckv1.CloudEventOverrides
	}{
		"TestAddRunRemoveSchedule": {
			wantCEOverrides: nil,
		}, "TestAddRunRemoveScheduleWithExtensionOverride": {
			wantCEOverrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{"1": "one", "2": "two"}},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			logger := logtesting.TestLogger(t)
			ce := adaptertesting.NewTestClient()

			runner := NewCronJobsRunner(ce, logger)

			entryId, err := runner.AddSchedule("test-ns", "test-name", "* * * * ?", "some data",
				"a sink", tc.wantCEOverrides)

			if err != nil {
				t.Errorf("Should not throw error %v", err)
			}

			entry := runner.cron.Entry(entryId)
			if entry.ID != entryId {
				t.Error("Entry has not been added")
			}

			entry.Job.Run()

			validateSent(t, ce, `{"body":"some data"}`, tc.wantCEOverrides)

			runner.RemoveSchedule(entryId)

			entry = runner.cron.Entry(entryId)
			if entry.ID == entryId {
				t.Error("Entry has not been removed")
			}
		})
	}
}

func TestStartStopCron(t *testing.T) {
	logger := logtesting.TestLogger(t)
	ce := adaptertesting.NewTestClient()

	runner := NewCronJobsRunner(ce, logger)

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

func validateSent(t *testing.T, ce *adaptertesting.TestCloudEventsClient, wantData string,
	wantCEOverrides *duckv1.CloudEventOverrides) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Data(); string(got) != wantData {
		t.Errorf("Expected %q event to be sent, got %q", wantData, got)
	}

	gotExtensions := ce.Sent()[0].Context.GetExtensions()

	if wantCEOverrides == nil && gotExtensions != nil {
		t.Errorf("Expected event with no extention overrides got %v", gotExtensions)
	}

	if wantCEOverrides != nil && gotExtensions == nil {
		t.Errorf("Expected event with extention overrides got nil")
	}
	if wantCEOverrides != nil {
		compareTo := map[string]interface{}{}
		for k, v := range wantCEOverrides.Extensions {
			compareTo[k] = v
		}
		if !reflect.DeepEqual(compareTo, gotExtensions) {
			t.Errorf("Expected event with extention overrides to be the same want: %v, but got: %v", wantCEOverrides, gotExtensions)
		}
	}
}
