/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

// Package test contains test data and generic tests for testing bindings.
package test

import (
	"fmt"
	"net/url"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/types"
)

func strptr(s string) *string { return &s }

var (
	Source    = types.URIRef{URL: url.URL{Scheme: "http", Host: "example.com", Path: "/source"}}
	Timestamp = types.Timestamp{Time: time.Date(2020, 03, 21, 12, 34, 56, 780000000, time.UTC)}
	Schema    = types.URI{URL: url.URL{Scheme: "http", Host: "example.com", Path: "/schema"}}
)

// FullEvent has all context attributes set and JSON string data.
func FullEvent() event.Event {
	e := event.Event{
		Context: event.EventContextV1{
			Type:       "com.example.FullEvent",
			Source:     Source,
			ID:         "full-event",
			Time:       &Timestamp,
			DataSchema: &Schema,
			Subject:    strptr("topic"),
		}.AsV1(),
	}

	e.SetExtension("exbool", true)
	e.SetExtension("exint", 42)
	e.SetExtension("exstring", "exstring")
	e.SetExtension("exbinary", []byte{0, 1, 2, 3})
	e.SetExtension("exurl", Source)
	e.SetExtension("extime", Timestamp)

	if err := e.SetData("text/json", "hello"); err != nil {
		panic(err)
	}
	return e
}

// MinEvent has only required attributes set.
func MinEvent() event.Event {
	return event.Event{
		Context: event.EventContextV1{
			Type:   "com.example.MinEvent",
			Source: Source,
			ID:     "min-event",
		}.AsV1(),
	}
}

// FullMessage returns the same event of FullEvent but wrapped as Message.
func FullMessage() binding.Message {
	ev := FullEvent()
	return binding.ToMessage(&ev)
}

// MinMessage returns the same event of MinEvent but wrapped as Message.
func MinMessage() binding.Message {
	ev := MinEvent()
	return binding.ToMessage(&ev)
}

// AllVersions returns all versions of each event in events.
// ID gets a -number suffix so IDs are unique.
func AllVersions(events []event.Event) []event.Event {
	versions := spec.New()
	all := versions.Versions()
	result := make([]event.Event, len(events)*len(all))
	i := 0
	for _, e := range events {
		for _, v := range all {
			result[i] = e
			result[i].Context = v.Convert(e.Context)
			result[i].SetID(fmt.Sprintf("%v-%v", e.ID(), i)) // Unique IDs
			i++
		}
	}
	return result
}

// Events is a set of test events that should be handled correctly by
// all event-processing code.
func Events() []event.Event {
	return AllVersions([]event.Event{FullEvent(), MinEvent()})
}
