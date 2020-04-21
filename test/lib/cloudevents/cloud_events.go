/*
Copyright 2019 The Knative Authors

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

package cloudevents

import (
	ce "github.com/cloudevents/sdk-go/v2"
)

// CloudEvent related constants.
const (
	DefaultEncoding = "binary"
	DefaultSource   = "http://knative.test"
	DefaultType     = "dev.knative.test.event"
)

// CloudEvent specifies the arguments for a CloudEvent used as input to create a new container in test.
type CloudEvent struct {
	Event    ce.Event
	Data     string // must be in json format
	Encoding string // binary or structured
}

// Option enables further configuration of a CloudEvent.
type Option func(*CloudEvent)

// WithType returns an option that changes the id for the given CloudEvent.
func WithID(id string) Option {
	return func(c *CloudEvent) {
		c.Event.SetID(id)
	}
}

// WithSource returns an option that changes the source for the given CloudEvent.
func WithSource(eventSource string) Option {
	return func(c *CloudEvent) {
		c.Event.SetSource(eventSource)
	}
}

// WithType returns an option that changes the type for the given CloudEvent.
func WithType(eventType string) Option {
	return func(c *CloudEvent) {
		c.Event.SetType(eventType)
	}
}

// WithType returns an option that changes the encoding for the given CloudEvent.
func WithEncoding(encoding string) Option {
	return func(c *CloudEvent) {
		c.Encoding = encoding
	}
}

// WithExtensions returns an option that changes the extensions for the given CloudEvent.
func WithExtensions(extensions map[string]interface{}) Option {
	return func(c *CloudEvent) {
		for k, v := range extensions {
			c.Event.SetExtension(k, v)
		}
	}
}

// New returns a new CloudEvent with most preset default properties.
func New(data string, options ...Option) *CloudEvent {
	event := &CloudEvent{
		Event:    ce.NewEvent(),
		Data:     data,
		Encoding: DefaultEncoding,
	}
	event.Event.SetSource(DefaultSource)
	event.Event.SetType(DefaultType)
	for _, option := range options {
		option(event)
	}
	return event
}

// BaseData defines a simple struct that can be used as data of a CloudEvent.
type BaseData struct {
	Sequence int    `json:"id"`
	Message  string `json:"message"`
}
