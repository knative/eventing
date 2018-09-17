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
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

const correlationIDHeaderName = "Knative-Correlation-Id"

// MessageDispatcher dispatches messages to a destination over HTTP.
type MessageDispatcher struct {
	httpClient       *http.Client
	forwardHeaders   map[string]bool
	forwardPrefixes  []string
	supportedSchemes map[string]bool

	logger *zap.SugaredLogger
}

// DispatchDefaults provides default parameter values used when dispatching a message.
type DispatchDefaults struct {
	Namespace string
	ReplyTo   string
}

// NewMessageDispatcher creates a new message dispatcher that can dispatch
// messages to HTTP destinations.
func NewMessageDispatcher(logger *zap.SugaredLogger) *MessageDispatcher {
	return &MessageDispatcher{
		httpClient:      &http.Client{},
		forwardHeaders:  headerSet(forwardHeaders),
		forwardPrefixes: forwardPrefixes,
		supportedSchemes: map[string]bool{
			"http":  true,
			"https": true,
		},

		logger: logger,
	}
}

// DispatchMessage dispatches a message to a destination over HTTP.
//
// The destination and replyTo are DNS names. For names with a single label,
// the default namespace is used to expand it into a fully qualified name
// within the cluster.
func (d *MessageDispatcher) DispatchMessage(message *Message, destination string, defaults DispatchDefaults) error {
	destinationURL := d.resolveURL(destination, defaults.Namespace)
	reply, err := d.executeRequest(destinationURL, message)
	if err != nil {
		return fmt.Errorf("Unable to complete request %v", err)
	}
	if defaults.ReplyTo != "" && reply != nil {
		replyToURL := d.resolveURL(defaults.ReplyTo, defaults.Namespace)
		_, err = d.executeRequest(replyToURL, reply)
		if err != nil {
			return fmt.Errorf("Failed to forward reply %v", err)
		}
	}
	return nil
}

func (d *MessageDispatcher) executeRequest(url *url.URL, message *Message) (*Message, error) {
	d.logger.Infof("Dispatching message to %s", url.String())
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(message.Payload))
	if err != nil {
		return nil, fmt.Errorf("Unable to create request %v", err)
	}
	req.Header = d.toHTTPHeaders(message.Headers)
	res, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res != nil {
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			// reject non-successful (2xx) responses
			return nil, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", res.StatusCode)
		}
		headers := d.fromHTTPHeaders(res.Header)
		// TODO: add configurable whitelisting of propagated headers/prefixes (configmap?)
		if correlationID, ok := message.Headers[correlationIDHeaderName]; ok {
			headers[correlationIDHeaderName] = correlationID
		}
		payload, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("Unable to read response %v", err)
		}
		return &Message{headers, payload}, nil
	}
	return nil, nil
}

// toHTTPHeaders converts message headers to HTTP headers.
//
// Only headers whitelisted as safe are copied.
func (d *MessageDispatcher) toHTTPHeaders(headers map[string]string) http.Header {
	safe := http.Header{}

	for name, value := range headers {
		// Header names are case insensitive. Be sure to compare against a lower-cased version
		// (all our oracles are lower-case as well).
		name = strings.ToLower(name)
		if _, ok := d.forwardHeaders[name]; ok {
			safe.Add(name, value)
			continue
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

// fromHTTPHeaders converts HTTP headers into a message header map.
//
// Only headers whitelisted as safe are copied. If an HTTP header exists
// multiple times, a single value will be retained.
func (d *MessageDispatcher) fromHTTPHeaders(headers http.Header) map[string]string {
	safe := map[string]string{}

	// TODO handle multi-value headers
	for h, v := range headers {
		// Headers are case-insensitive but test case are all lower-case
		comparable := strings.ToLower(h)
		if _, ok := d.forwardHeaders[comparable]; ok {
			safe[h] = v[0]
			continue
		}
		for _, p := range d.forwardPrefixes {
			if strings.HasPrefix(comparable, p) {
				safe[h] = v[0]
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
