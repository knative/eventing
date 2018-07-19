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
	httpClient       *http.Client
	forwardHeaders   map[string]bool
	forwardPrefixes  []string
	supportedSchemes map[string]bool
}

// NewMessageDispatcher creates a new message dispatcher that can dispatch
// messages to HTTP destinations.
func NewMessageDispatcher() *MessageDispatcher {
	return &MessageDispatcher{
		httpClient:      &http.Client{},
		forwardHeaders:  headerSet(forwardHeaders),
		forwardPrefixes: forwardPrefixes,
		supportedSchemes: map[string]bool{
			"http":  true,
			"https": true,
		},
	}
}

// DispatchMessage dispatches a message to a destination over HTTP.
//
// The destination is a DNS name. For destinations with a single label, the
// default namespace is used to expand the destination into a fully qualified
// name within the cluster.
func (d *MessageDispatcher) DispatchMessage(destination string, defaultNamespace string, message *Message) error {
	url := d.resolveURL(destination, defaultNamespace)
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(message.Payload))
	if err != nil {
		return fmt.Errorf("Unable to create request %v", err)
	}
	req.Header = d.toHTTPHeaders(message.Headers)
	_, err = d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Unable to complete request %v", err)
	}
	return nil
}

// toHTTPHeaders converts message headers to HTTP headers.
//
// Only headers whitelisted as safe are copied.
func (d *MessageDispatcher) toHTTPHeaders(headers map[string]string) http.Header {
	safe := http.Header{}

	for name, value := range headers {
		if _, ok := d.forwardHeaders[name]; ok {
			safe.Add(name, value)
			break
		}
		for _, prefix := range d.forwardPrefixes {
			if strings.HasPrefix(name, prefix) {
				safe.Add(name, value)
				break
			}
		}
	}

	return safe
}

func (d *MessageDispatcher) resolveURL(destination string, defaultNamespace string) *url.URL {
	if url, err := url.Parse(destination); err == nil && d.supportedSchemes[url.Scheme] {
		// already a URL with a known scheme
		return url
	}
	if strings.Index(destination, ".") == -1 {
		destination = fmt.Sprintf("%s.%s.svc.cluster.local", destination, defaultNamespace)
	}
	return &url.URL{
		Scheme: "http",
		Host:   destination,
		Path:   "/",
	}
}
