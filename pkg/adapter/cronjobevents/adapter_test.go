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

package cronjobevents

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/source"
)

type mockReporter struct {
	eventCount int
}

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount++
	return nil
}

func TestStart_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		schedule string
		sink     func(http.ResponseWriter, *http.Request)
		reqBody  string
		error    bool
	}{
		"happy": {
			schedule: "* * * * *", // every minute
			sink:     sinkAccepted,
			reqBody:  `{"body":"data"}`,
		},
		"rejected": {
			schedule: "* * * * *", // every minute
			sink:     sinkRejected,
			reqBody:  `{"body":"data"}`,
			error:    true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			r := &mockReporter{}
			a := &Adapter{
				Schedule: tc.schedule,
				Data:     "data",
				SinkURI:  sinkServer.URL,
				Reporter: r,
			}

			if err := a.initClient(); err != nil {
				t.Errorf("failed to create cloudevent client, %v", err)
			}

			stop := make(chan struct{})
			go func() {
				if err := a.Start(context.TODO(), stop); err != nil {
					if tc.error {
						// skip
					} else {
						t.Errorf("failed to start, %v", err)
					}
				}
			}()

			a.cronTick() // force a tick.
			validateMetric(t, a.Reporter, 1)

			if tc.reqBody != string(h.body) {
				t.Errorf("expected request body %q, but got %q", tc.reqBody, h.body)
			}
			log.Print("test done")
		})
	}
}

func TestStartBadCron(t *testing.T) {
	schedule := "bad"

	r := &mockReporter{}
	a := &Adapter{
		Schedule: schedule,
		Reporter: r,
	}

	stop := make(chan struct{})
	if err := a.Start(context.TODO(), stop); err == nil {

		t.Errorf("failed to fail, %v", err)

	}

	validateMetric(t, a.Reporter, 0)
}

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink    func(http.ResponseWriter, *http.Request)
		reqBody string
		error   bool
	}{
		"happy": {
			sink:    sinkAccepted,
			reqBody: `{"body":"data"}`,
		},
		"rejected": {
			sink:    sinkRejected,
			reqBody: `{"body":"data"}`,
			error:   true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			r := &mockReporter{}
			a := &Adapter{
				Data:     "data",
				SinkURI:  sinkServer.URL,
				Reporter: r,
			}

			if err := a.initClient(); err != nil {
				t.Errorf("failed to create cloudevent client, %v", err)
			}

			a.cronTick()

			if tc.reqBody != string(h.body) {
				t.Errorf("expected request body %q, but got %q", tc.reqBody, h.body)
			}
			validateMetric(t, a.Reporter, 1)
		})
	}
}

func TestMessage(t *testing.T) {
	testCases := map[string]struct {
		body string
		want string
	}{
		"json simple": {
			body: `{"message": "Hello world!"}`,
			want: `{"message":"Hello world!"}`,
		},
		"json complex": {
			body: `{"message": "Hello world!","extra":{"a":"sub", "b":[1,2,3]}}`,
			want: `{"extra":{"a":"sub","b":[1,2,3]},"message":"Hello world!"}`,
		},
		"string": {
			body: "Hello, World!",
			want: `{"body":"Hello, World!"}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			m := message(tc.body)

			j, err := json.Marshal(m)
			if err != nil {
				t.Errorf("failed to marshel message: %v", err)
			}

			got := string(j)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s: (-want, +got) = %v", n, diff)
			}
		})
	}
}

type fakeHandler struct {
	body    []byte
	ran     int
	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.handler(w, r)

	h.ran++
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}

func validateMetric(t *testing.T, reporter source.StatsReporter, want int) {
	if mockReporter, ok := reporter.(*mockReporter); !ok {
		t.Errorf("reporter is not a mockReporter")
	} else if mockReporter.eventCount != want {
		t.Errorf("Expected %d for metric, got %d", want, mockReporter.eventCount)
	}
}
