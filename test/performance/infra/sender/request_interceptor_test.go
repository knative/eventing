/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sender

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestInterceptor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()

	var calledBefore, calledAfter bool

	ti := requestInterceptor{
		before: func(*http.Request) {
			calledBefore = true
		},
		transport: http.DefaultTransport,
		after: func(*http.Request, *http.Response, error) {
			calledAfter = true
		},
	}

	tc := http.Client{Transport: ti}

	if _, err := tc.Get(ts.URL); err != nil {
		t.Fatal("Failed to send request to mock server:", err)
	}

	if !calledBefore || !calledAfter {
		t.Errorf("Expected calls to before and after funcs (before: %t, after: %t)", calledBefore, calledAfter)
	}
}
