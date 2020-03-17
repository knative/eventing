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
	"regexp"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
)

const (
	// EventHistory is the header containing all channel hosts traversed by the event.
	// This is an experimental header: https://github.com/knative/eventing/issues/638.
	EventHistory          = "knativehistory"
	EventHistorySeparator = "; "
)

var historySplitter = regexp.MustCompile(`\s*` + regexp.QuoteMeta(EventHistorySeparator) + `\s*`)

// AppendToHistory appends a new host at the end of the list of hosts of the event history.
func AppendHistory(event *cloudevents.Event, host string) {
	host = cleanupEventHistoryItem(host)
	if host == "" {
		return
	}
	h := history(event)
	setHistory(event, append(h, host))
}

// history returns the list of hosts where the event has been into.
func history(event *cloudevents.Event) []string {
	var h string
	if err := event.ExtensionAs(EventHistory, &h); err != nil {
		return nil
	}
	return decodeEventHistory(h)
}

// setHistory sets the event history to the given value.
func setHistory(event *cloudevents.Event, history []string) {
	event.SetExtension(EventHistory, encodeEventHistory(history))
}

func cleanupEventHistoryItem(host string) string {
	return strings.Trim(host, " ")
}

func encodeEventHistory(history []string) string {
	return strings.Join(history, EventHistorySeparator)
}

func decodeEventHistory(historyStr string) []string {
	readHistory := historySplitter.Split(historyStr, -1)
	// Filter and cleanup in-place
	history := readHistory[:0]
	for _, item := range readHistory {
		cleanItem := cleanupEventHistoryItem(item)
		if cleanItem != "" {
			history = append(history, cleanItem)
		}
	}
	return history
}
