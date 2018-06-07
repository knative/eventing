/*
Copyright 2018 Google, Inc. All rights reserved.

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

package event

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const (
	// HeaderCloudEventsVersion is the header for the version of Cloud Events
	// used.
	HeaderCloudEventsVersion = "cloud-events-version"

	// HeaderEventID is the header for the unique ID of this event.
	HeaderEventID = "event-id"

	// HeaderEventType is the header for type of event represented. Value SHOULD
	// be in reverse-dns form.
	HeaderEventType = "event-type"

	// HeaderEventTime is the OPTIONAL header for the time at which an event
	// occurred.
	HeaderEventTime = "event-time"

	// HeaderSource is the header for the source which emitted this event.
	HeaderSource = "source"

	fieldCloudEventsVersion = "CloudEventsVersion"
	fieldEventID            = "EventID"
	fieldEventType          = "EventType"
	fieldEventTime          = "EventTime"
	fieldSource             = "Source"
)

// Context holds standard metadata about an event.
type Context struct {
	CloudEventsVersion string    `json:"cloud-events-version,omitempty"`
	EventID            string    `json:"event-id"`
	EventType          string    `json:"event-type"`
	EventTime          time.Time `json:"event-time,omitempty"`
	Source             string    `json:"source"`
}

func anyError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func pullReqHeader(h http.Header, name string, value *string) error {
	if *value = h.Get(name); *value == "" {
		return fmt.Errorf("missing required header %q", name)
	}
	return nil
}

func require(name string, value string) error {
	if len(value) == 0 {
		return fmt.Errorf("missing required field %q", name)
	}
	return nil
}

// FromRequest parses event data and context from an HTTP request.
func FromRequest(data interface{}, r *http.Request) (*Context, error) {
	var ctx Context
	err := anyError(
		pullReqHeader(r.Header, HeaderEventID, &ctx.EventID),
		pullReqHeader(r.Header, HeaderEventType, &ctx.EventType),
		pullReqHeader(r.Header, HeaderSource, &ctx.Source))
	if err != nil {
		return nil, err
	}

	ctx.CloudEventsVersion = r.Header.Get(HeaderCloudEventsVersion)
	if timeStr := r.Header.Get(HeaderEventTime); timeStr != "" {
		if err := ctx.EventTime.UnmarshalText([]byte(timeStr)); err != nil {
			return nil, err
		}
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return nil, err
	}

	return &ctx, nil
}

// NewRequest creates an HTTP request for a given event data and context.
func NewRequest(urlString string, data interface{}, context Context) (*http.Request, error) {
	url, err := url.Parse(urlString)
	err = anyError(
		err,
		require(fieldEventID, context.EventID),
		require(fieldEventType, context.EventType),
		require(fieldSource, context.Source))
	if err != nil {
		return nil, err
	}

	h := http.Header{}
	h.Set(HeaderEventID, context.EventID)
	h.Set(HeaderEventType, context.EventType)
	h.Set(HeaderSource, context.Source)

	if context.CloudEventsVersion != "" {
		h.Set(HeaderCloudEventsVersion, context.CloudEventsVersion)
	}
	if !context.EventTime.IsZero() {
		b, err := context.EventTime.UTC().MarshalText()
		if err != nil {
			return nil, err
		}
		h.Set(HeaderEventTime, string(b))
	}

	buffer, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &http.Request{
		Method: http.MethodPost,
		URL:    url,
		Header: h,
		Body:   ioutil.NopCloser(bytes.NewReader(buffer)),
	}, nil
}
