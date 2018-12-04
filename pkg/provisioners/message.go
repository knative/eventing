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
