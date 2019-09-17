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

package channel

import (
	"errors"
	"github.com/cloudevents/sdk-go"
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

// ErrUnknownChannel is returned when a message is received by a channel dispatcher for a
// channel that does not exist.
var ErrUnknownChannel = errors.New("unknown channel")

// History returns the list of hosts where an event has been into.
func History(tctx cloudevents.HTTPTransportContext) []string {
	if tctx.Header == nil {
		return nil
	}
	if h, ok := tctx.Header[MessageHistoryHeader]; ok {
		return decodeMessageHistory(h[0])
	}
	return nil
}

// AppendToHistory appends a new host at the end of the list of hosts of the event history.
func AppendToHistory(tctx cloudevents.HTTPTransportContext, history []string, host string) {
	host = cleanupMessageHistoryItem(host)
	if host == "" {
		return
	}
	setHistory(append(tctx, history, host)))
}

// setHistory sets the message history to the given value
func setHistory(tctx cloudevents.HTTPTransportContext, history []string) {
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
