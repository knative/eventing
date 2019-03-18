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

package provisioners

import (
	"errors"
	"regexp"
	"strings"
)

const (
	// MessageHistoryHeader is the header containing all channel hosts traversed by the message
	// This is an experimental header: https://github.com/knative/eventing/issues/638
	MessageHistoryHeader    = "ce-knativehistory"
	MessageHistorySeparator = "; "
)

var historySplitter = regexp.MustCompile(`\s*` + regexp.QuoteMeta(MessageHistorySeparator) + `\s*`)

var forwardHeaders = []string{
	"content-type",
	// tracing
	"x-request-id",
}

var forwardPrefixes = []string{
	// knative
	"knative-",
	// cloud events
	"ce-",
	// tracing
	"x-b3-",
	"x-ot-",
}

// Message represents an chunk of data within a channel dispatcher. The message contains both
// a map of string headers and a binary payload. This struct gets mashaled/unmarshaled in order to
// preserve and pass Header information to the event subscriber.
//
// A message may represent a CloudEvent.
type Message struct {
	// Headers provide metadata about the message payload. All header keys
	// should be lowercase.
	Headers map[string]string `json:"headers,omitempty"`

	// Payload is the raw binary content of the message. The payload format is
	// often described by the 'content-type' header.
	Payload []byte `json:"payload,omitempty"`
}

// ErrUnknownChannel is returned when a message is received by a channel dispatcher for a
// channel that does not exist.
var ErrUnknownChannel = errors.New("unknown channel")

// History returns the list of hosts where the message has been into
func (m *Message) History() []string {
	if m.Headers == nil {
		return nil
	}
	if h, ok := m.Headers[MessageHistoryHeader]; ok {
		return decodeMessageHistory(h)
	}
	return nil
}

// AppendToHistory append a new host at the end of the list of hosts of the message history
func (m *Message) AppendToHistory(host string) {
	host = cleanupMessageHistoryItem(host)
	if host == "" {
		return
	}
	m.setHistory(append(m.History(), host))
}

// setHistory sets the message history to the given value
func (m *Message) setHistory(history []string) {
	historyStr := encodeMessageHistory(history)
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[MessageHistoryHeader] = historyStr
}

func cleanupMessageHistoryItem(host string) string {
	return strings.Trim(host, " ")
}

func encodeMessageHistory(history []string) string {
	return strings.Join(history, MessageHistorySeparator)
}

func decodeMessageHistory(historyStr string) []string {
	readHistory := historySplitter.Split(historyStr, -1)
	// Filter and cleanup in-place
	history := readHistory[:0]
	for _, item := range readHistory {
		cleanItem := cleanupMessageHistoryItem(item)
		if cleanItem != "" {
			history = append(history, cleanItem)
		}
	}
	return history
}
