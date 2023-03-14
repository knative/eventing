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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const ceClientURL = "http://localhost:8080"

func TestRun_HealthEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	go run(ctx)
	if err := waitForClient(ctx); err != nil {
		t.Fatal("Error waiting for CloudEvents receiver:", err)
	}

	const healthzURL = ceClientURL + healthzPath
	const expectStatusCode = http.StatusNoContent

	// GET request
	resp, err := http.Get(healthzURL)
	if err != nil {
		t.Fatal("Error sending GET request to health endpoint:", err)
	}
	if gotStatusCode := resp.StatusCode; gotStatusCode != expectStatusCode {
		t.Error("Unexpected status code sending GET request to health endpoint:", gotStatusCode)
	}

	// POST request
	resp, err = http.Post(healthzURL, "text/plain", new(bytes.Buffer))
	if err != nil {
		t.Fatal("Error sending POST request to health endpoint:", err)
	}
	if gotStatusCode := resp.StatusCode; gotStatusCode != expectStatusCode {
		t.Error("Unexpected status code sending POST request to health endpoint:", gotStatusCode)
	}
}

// waitForClient sends requests to the local CloudEvents receiver address until
// a HTTP response is received, or until ctx is cancelled.
func waitForClient(ctx context.Context) error {
	httpClient := http.Client{}
	var httpErr error

	tick := time.Tick(5 * time.Millisecond)

	for {
		select {
		case <-tick:
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, ceClientURL, nil)
			if err != nil {
				return fmt.Errorf("creating HTTP request: %w", err)
			}

			if _, httpErr = httpClient.Do(req); httpErr == nil {
				// got a response from CloudEvents client's HTTP receiver
				return nil
			}

		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %s. The last HTTP error was: %s", ctx.Err(), httpErr)
		}
	}
}

func TestLogRequest(t *testing.T) {
	bodyContent := "hello"
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(bodyContent)
	req, err := http.NewRequest("POST", "https://localhost", buffer)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("content-type", "application/json")

	logRequest(req)

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != bodyContent {
		t.Fatal("got", string(body), "want", bodyContent)
	}
}
