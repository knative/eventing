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
	"encoding/json"
	"errors"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/api/core/v1"
)

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

const (
	// messageHistoryHeader is the header containing all channels in the message history
	messageHistoryHeader = "knative-message-history"
)

// Message represents an chunk of data within a channel dispatcher. The message contains both
// a map of string headers and a binary payload.
//
// A message may represent a CloudEvent.
type Message struct {
	// Headers provide metadata about the message payload. All header keys
	// should be lowercase.
	Headers map[string]string

	// Payload is the raw binary content of the message. The payload format is
	// often described by the 'content-type' header.
	Payload []byte
}

// ErrUnknownChannel is returned when a message is received by a channel dispatcher for a
// channel that does not exist.
var ErrUnknownChannel = errors.New("unknown channel")

func headerSet(headers []string) map[string]bool {
	set := make(map[string]bool)
	for _, header := range headers {
		set[header] = true
	}
	return set
}

// History returns the list channel references where the message has been into
func (m *Message) History() ([]v1.ObjectReference, error) {
	if m.Headers == nil {
		return nil, nil
	}
	if h, ok := m.Headers[messageHistoryHeader]; ok {
		return decodeMessageHistory(h)
	}
	return nil, nil
}

// AppendToHistory append a new channel reference at the end of the list of channel references of the message
func (m *Message) AppendToHistory(reference ChannelReference, overwriteUnreadable bool) error {
	objectReference := v1.ObjectReference{
		Name:       reference.Name,
		Namespace:  reference.Namespace,
		Kind:       "Channel",
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
	}
	history, err := m.History()
	if err != nil && !overwriteUnreadable {
		return err
	} else if err != nil {
		// overwrite unreadable data
		history = make([]v1.ObjectReference, 0)
	}
	newHistory := append(history, objectReference)
	return m.SetHistory(newHistory)
}

// SetHistory sets the message history to the given value
func (m *Message) SetHistory(history []v1.ObjectReference) error {
	historyStr, err := encodeMessageHistory(history)
	if err != nil {
		return err
	}
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[messageHistoryHeader] = historyStr
	return nil
}

func encodeMessageHistory(history []v1.ObjectReference) (string, error) {
	data, err := json.Marshal(history)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeMessageHistory(historyStr string) ([]v1.ObjectReference, error) {
	history := make([]v1.ObjectReference, 0)
	err := json.Unmarshal([]byte(historyStr), &history)
	return history, err
}
