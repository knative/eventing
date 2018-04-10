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

// Context holds standard metadata about an event.
type Context struct {
	CloudEventsVersion string    `json:"cloud-events-version,omitempty"`
	EventId            string    `json":event-id"`
	EventType          string    `json:"event-type"`
	EventTime          time.Time `json:"event-time,omitempty"`
	SourceAuthority    string    `json:"source-authority"`
	SourcePath         string    `json:"source-path,omitempty"`
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
		return fmt.Errorf("missing required header '%s'", name)
	}
	return nil
}

func require(name string, value string) error {
	if len(value) == 0 {
		return fmt.Errorf("missing required field '%s'", name)
	}
	return nil
}

// FromRequest parses event data and context from an HTTP request.
func FromRequest(data interface{}, r *http.Request) (*Context, error) {
	var ctx Context
	err := anyError(
		pullReqHeader(r.Header, "event-id", &ctx.EventId),
		pullReqHeader(r.Header, "event-type", &ctx.EventType),
		pullReqHeader(r.Header, "source-authority", &ctx.SourceAuthority),
		pullReqHeader(r.Header, "source-path", &ctx.SourcePath))
	if err != nil {
		return nil, err
	}

	ctx.CloudEventsVersion = r.Header.Get("cloud-events-version")
	var timeStr = r.Header.Get("event-time")
	if timeStr != "" {
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
		require("EventId", context.EventId),
		require("EventType", context.EventType),
		require("SourceAuthority", context.SourceAuthority),
		require("SourcePath", context.SourcePath))
	if err != nil {
		return nil, err
	}

	h := http.Header{}
	h.Set("event-id", context.EventId)
	h.Set("event-type", context.EventType)
	h.Set("source-authority", context.SourceAuthority)
	h.Set("source-path", context.SourcePath)

	if context.CloudEventsVersion != "" {
		h.Set("cloud-events-version", context.CloudEventsVersion)
	}
	if !context.EventTime.IsZero() {
		b, err := context.EventTime.UTC().MarshalText()
		if err != nil {
			return nil, err
		}
		h.Set("event-time", string(b))
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
