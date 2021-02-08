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
	"bytes"
	"context"
	"encoding/base64"
	"reflect"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	_ "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/logging"
	rectesting "knative.dev/pkg/reconciler/testing"

	adaptertesting "knative.dev/eventing/pkg/adapter/v2/test"
	"knative.dev/eventing/pkg/apis/sources/v1beta2"
)

const (
	threeSecondsTillNextMinCronJob = 60 - 3
	sampleData                     = "some data"
	sampleJSONData                 = `{"msg":"some data"}`
	sampleXmlData                  = "<pre>Value</pre>"
	sampleDataBase64               = "c29tZSBkYXRh"                 // "some data"
	sampleJSONDataBase64           = "eyJtc2ciOiJzb21lIGRhdGEifQ==" // {"msg":"some data"}
)

func decodeBase64(base64Str string) []byte {
	decoded, _ := base64.StdEncoding.DecodeString(base64Str)
	return decoded
}

func TestAddRunRemoveSchedules(t *testing.T) {
	testCases := map[string]struct {
		src             *v1beta2.PingSource
		wantContentType string
		wantData        []byte
	}{
		"TestAddRunRemoveSchedule": {
			src: &v1beta2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Spec: v1beta2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: &duckv1.CloudEventOverrides{},
					},
					Schedule:    "* * * * ?",
					ContentType: cloudevents.TextPlain,
					Data:        sampleData,
				},
				Status: v1beta2.PingSourceStatus{
					SourceStatus: duckv1.SourceStatus{
						SinkURI: &apis.URL{Path: "a sink"},
					},
				},
			},
			wantData:        []byte(sampleData),
			wantContentType: cloudevents.TextPlain,
		}, "TestAddRunRemoveScheduleWithExtensionOverride": {
			src: &v1beta2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Spec: v1beta2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: &duckv1.CloudEventOverrides{
							Extensions: map[string]string{"1": "one", "2": "two"},
						},
					},
					Schedule:    "* * * * ?",
					ContentType: cloudevents.TextPlain,
					Data:        sampleData,
				},
				Status: v1beta2.PingSourceStatus{
					SourceStatus: duckv1.SourceStatus{
						SinkURI: &apis.URL{Path: "a sink"},
					},
				},
			},
			wantData:        []byte(sampleData),
			wantContentType: cloudevents.TextPlain,
		}, "TestAddRunRemoveScheduleWithDataBase64": {
			src: &v1beta2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Spec: v1beta2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: &duckv1.CloudEventOverrides{},
					},
					Schedule:    "* * * * ?",
					ContentType: cloudevents.TextPlain,
					DataBase64:  sampleDataBase64,
				},
				Status: v1beta2.PingSourceStatus{
					SourceStatus: duckv1.SourceStatus{
						SinkURI: &apis.URL{Path: "a sink"},
					},
				},
			},
			wantData:        decodeBase64(sampleDataBase64),
			wantContentType: cloudevents.TextPlain,
		}, "TestAddRunRemoveScheduleWithJsonData": {
			src: &v1beta2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Spec: v1beta2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: &duckv1.CloudEventOverrides{},
					},
					Schedule:    "* * * * ?",
					Data:        sampleJSONData,
					ContentType: cloudevents.ApplicationJSON,
				},
				Status: v1beta2.PingSourceStatus{
					SourceStatus: duckv1.SourceStatus{
						SinkURI: &apis.URL{Path: "a sink"},
					},
				},
			},
			wantData:        []byte(sampleJSONData),
			wantContentType: cloudevents.ApplicationJSON,
		}, "TestAddRunRemoveScheduleWithJsonDataBase64": {
			src: &v1beta2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Spec: v1beta2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: &duckv1.CloudEventOverrides{},
					},
					Schedule:    "* * * * ?",
					DataBase64:  sampleJSONDataBase64,
					ContentType: cloudevents.ApplicationJSON,
				},
				Status: v1beta2.PingSourceStatus{
					SourceStatus: duckv1.SourceStatus{
						SinkURI: &apis.URL{Path: "a sink"},
					},
				},
			},
			wantData:        decodeBase64(sampleJSONDataBase64),
			wantContentType: cloudevents.ApplicationJSON,
		}, "TestAddRunRemoveScheduleWithXmlData": {
			src: &v1beta2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Spec: v1beta2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: &duckv1.CloudEventOverrides{},
					},
					Schedule:    "* * * * ?",
					Data:        sampleXmlData,
					ContentType: cloudevents.ApplicationXML,
				},
				Status: v1beta2.PingSourceStatus{
					SourceStatus: duckv1.SourceStatus{
						SinkURI: &apis.URL{Path: "a sink"},
					},
				},
			},
			wantData:        []byte(sampleXmlData),
			wantContentType: cloudevents.ApplicationXML,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx, _ := rectesting.SetupFakeContext(t)
			logger := logging.FromContext(ctx)
			ce := adaptertesting.NewTestClient()

			runner := NewCronJobsRunner(ce, kubeclient.Get(ctx), logger)
			entryId := runner.AddSchedule(tc.src)

			entry := runner.cron.Entry(entryId)
			if entry.ID != entryId {
				t.Error("Entry has not been added")
			}

			entry.Job.Run()

			validateSent(t, ce, tc.wantData, tc.wantContentType, tc.src.Spec.CloudEventOverrides.Extensions)

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
		runner.AddSchedule(
			&v1beta2.PingSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-ns",
				},
				Spec: v1beta2.PingSourceSpec{
					SourceSpec: duckv1.SourceSpec{
						CloudEventOverrides: &duckv1.CloudEventOverrides{},
					},
					Schedule:    "* * * * *",
					ContentType: cloudevents.TextPlain,
					Data:        "some delayed data",
				},
				Status: v1beta2.PingSourceStatus{
					SourceStatus: duckv1.SourceStatus{
						SinkURI: &apis.URL{Path: "a delayed sink"},
					},
				},
			})
		runner.Start(ctx.Done())
	}()

	tn = time.Now()
	seconds = tn.Second()

	time.Sleep(time.Second * (61 - time.Duration(seconds))) // one second past the minute

	runner.Stop() // cron job because of delay is still running.

	validateSent(t, ce, []byte("some delayed data"), cloudevents.TextPlain, nil)
}

func validateSent(t *testing.T, ce *adaptertesting.TestCloudEventsClient, wantData []byte, wantContentType string, extensions map[string]string) {
	if got := len(ce.Sent()); got != 1 {
		t.Error("Expected 1 event to be sent, got", got)
	}

	event := ce.Sent()[0]

	if gotContentType := event.DataContentType(); gotContentType != wantContentType {
		t.Errorf("Expected event with contentType=%q to be sent, got %q", wantContentType, gotContentType)
	}

	if got := event.Data(); !bytes.Equal(wantData, got) {
		t.Errorf("Expected %q event to be sent, got %q", wantData, got)
	}

	gotExtensions := event.Context.GetExtensions()

	if extensions == nil && gotExtensions != nil {
		t.Error("Expected event with no extension overrides, got:", gotExtensions)
	}

	if extensions != nil && gotExtensions == nil {
		t.Error("Expected event with extension overrides but got nil")
	}

	if extensions != nil {
		compareTo := make(map[string]interface{}, len(extensions))
		for k, v := range extensions {
			compareTo[k] = v
		}
		if !reflect.DeepEqual(compareTo, gotExtensions) {
			t.Errorf("Expected event with extension overrides to be the same want: %v, but got: %v", extensions, gotExtensions)
		}
	}
}
