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

	corev1 "k8s.io/api/core/v1"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	_ "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/logging"
	rectesting "knative.dev/pkg/reconciler/testing"

	adaptertesting "knative.dev/eventing/pkg/adapter/v2/test"
)

const threeSecondsTillNextMinCronJob = 60 - 3

func TestAddRunRemoveSchedules(t *testing.T) {
	testCases := map[string]struct {
		cfg   PingConfig
		delay time.Duration
	}{
		"TestAddRunRemoveSchedule": {
			cfg: PingConfig{
				ObjectReference: corev1.ObjectReference{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Schedule:   "* * * * ?",
				JsonData:   "some data",
				Extensions: nil,
				SinkURI:    "a sink",
			},
		}, "TestAddRunRemoveScheduleWithExtensionOverride": {
			cfg: PingConfig{
				ObjectReference: corev1.ObjectReference{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Schedule:   "* * * * ?",
				JsonData:   "some data",
				Extensions: map[string]string{"1": "one", "2": "two"},
				SinkURI:    "a sink",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx, _ := rectesting.SetupFakeContext(t)
			logger := logging.FromContext(ctx)
			ce := adaptertesting.NewTestClient()

			runner := NewCronJobsRunner(ce, kubeclient.Get(ctx), logger)
			entryId := runner.AddSchedule(tc.cfg)

			entry := runner.cron.Entry(entryId)
			if entry.ID != entryId {
				t.Error("Entry has not been added")
			}

			entry.Job.Run()

			validateSent(t, ce, `{"body":"some data"}`, tc.cfg.Extensions)

			runner.RemoveSchedule(entryId)

			entry = runner.cron.Entry(entryId)
			if entry.ID == entryId {
				t.Error("Entry has not been removed")
			}
		})
	}
}

func TestStartStopCron(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	logger := logging.FromContext(ctx)
	ce := adaptertesting.NewTestClient()

	runner := NewCronJobsRunner(ce, kubeclient.Get(ctx), logger)

	ctx, cancel := context.WithCancel(context.Background())
	wctx, wcancel := context.WithCancel(context.Background())

	go func() {
		runner.Start(ctx.Done())
		wcancel()
	}()

	cancel()

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("expected cron to be stopped after 2 seconds")
	case <-wctx.Done():
	}

}

func TestStartStopCronDelayWait(t *testing.T) {
	tn := time.Now()
	seconds := tn.Second()
	if seconds > threeSecondsTillNextMinCronJob {
		time.Sleep(time.Second * 4) // ward off edge cases
	}
	ctx, _ := rectesting.SetupFakeContext(t)
	logger := logging.FromContext(ctx)
	ce := adaptertesting.NewTestClientWithDelay(time.Second * 5)

	runner := NewCronJobsRunner(ce, kubeclient.Get(ctx), logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		runner.AddSchedule(PingConfig{
			ObjectReference: corev1.ObjectReference{
				Name:      "test-name",
				Namespace: "test-ns",
			},
			Schedule: "* * * * *",
			JsonData: "some delayed data",
			SinkURI:  "a delayed sink",
		})
		runner.Start(ctx.Done())

	}()

	tn = time.Now()
	seconds = tn.Second()

	time.Sleep(time.Second * (61 - time.Duration(seconds))) // one second past the minute

	runner.Stop() // cron job because of delay is still running.

	validateSent(t, ce, `{"body":"some delayed data"}`, nil)

}

func validateSent(t *testing.T, ce *adaptertesting.TestCloudEventsClient, wantData string,
	extensions map[string]string) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Data(); string(got) != wantData {
		t.Errorf("Expected %q event to be sent, got %q", wantData, got)
	}

	gotExtensions := ce.Sent()[0].Context.GetExtensions()

	if extensions == nil && gotExtensions != nil {
		t.Errorf("Expected event with no extension overrides got %v", gotExtensions)
	}

	if extensions != nil && gotExtensions == nil {
		t.Errorf("Expected event with extension overrides got nil")
	}

	if extensions != nil {
		compareTo := map[string]interface{}{}
		for k, v := range extensions {
			compareTo[k] = v
		}
		if !reflect.DeepEqual(compareTo, gotExtensions) {
			t.Errorf("Expected event with extension overrides to be the same want: %v, but got: %v", extensions, gotExtensions)
		}
	}
}

func configKey(key string, schedule string) string {
	return `"` + key + `"` + `: {
		"namespace": "default",
		"name":"ping",
		"schedule": "` + schedule + `",
		"jsonData":"{\"msg\": \"hello\"}",
		"sinkUri": "http://event-display.default.svc.cluster.local"}`
}
