/*
Copyright 2020 The Knative Authors

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

package channel

import (
	"regexp"
	"strings"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/types"
)

const (
	// EventHistory is the header containing all channel hosts traversed by the event.
	// This is an experimental header: https://github.com/knative/eventing/issues/638.
	EventHistory          = "knativehistory"
	EventHistorySeparator = "; "
)

var historySplitter = regexp.MustCompile(`\s*` + regexp.QuoteMeta(EventHistorySeparator) + `\s*`)

func AddHistory(host string) binding.Transformer {
	return transformer.SetExtension(EventHistory, func(i interface{}) (interface{}, error) {
		if types.IsZero(i) {
			return host, nil
		}
		str, err := types.Format(i)
		if err != nil {
			return nil, err
		}
		h := decodeEventHistory(str)
		h = append(h, host)
		return encodeEventHistory(h), nil
	})
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
