/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package buses

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// MessageDispatcher dispatches messages to a destination over HTTP.
type MessageDispatcher struct {
	httpClient     *http.Client
	forwardHeaders []string
}

// NewMessageDispatcher creates a new message dispatcher that can dispatch
// messages to HTTP destinations.
func NewMessageDispatcher() *MessageDispatcher {
	return &MessageDispatcher{
		httpClient:     &http.Client{},
		forwardHeaders: forwardHeaders,
	}
}

// DispatchMessage dispatches a message to a destination over HTTP.
//
// The destination is a DNS name. For destinations with a single label, the
// default namespace is used to expand the destination into a fully qualified
// name within the cluster.
func (d *MessageDispatcher) DispatchMessage(destination string, defaultNamespace string, message *Message) error {
	destination = d.resolveDestination(destination, defaultNamespace)
	url := url.URL{
		Scheme: "http",
		Host:   destination,
		Path:   "/",
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(message.Payload))
	if err != nil {
		return fmt.Errorf("Unable to create request %v", err)
	}
	for key, value := range message.Headers {
		req.Header.Add(key, value)
	}
	_, err = d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Unable to complete request %v", err)
	}
	return nil
}

func (d *MessageDispatcher) resolveDestination(destination string, defaultNamespace string) string {
	if strings.Index(destination, ".") == -1 {
		destination = fmt.Sprintf("%s.%s.svc.cluster.local", destination, defaultNamespace)
	}
	return destination
}
