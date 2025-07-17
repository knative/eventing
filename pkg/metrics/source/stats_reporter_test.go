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
)

func TestStatsReporter(t *testing.T) {
	args := &ReportArgs{
		Namespace:     "testns",
		EventType:     "dev.knative.event",
		EventSource:   "unit-test",
		Name:          "testsource",
		ResourceGroup: "testresourcegroup",
		EventScheme:   "http",
	}

	r, err := NewStatsReporter()
	if err != nil {
		t.Fatal("Failed to create a new reporter:", err)
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
}

func expectSuccess(t *testing.T, f func() error) {
	t.Helper()
	if err := f(); err != nil {
		t.Error("Reporter expected success but got error:", err)
	}
}
