/*
 Copyright 2022 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudevents/conformance/pkg/event"
)

type ResultsFn func(*http.Request, *http.Response, error)

func addHeader(req *http.Request, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		req.Header.Add(key, value)
	}
}

func addStructured(env map[string]interface{}, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		env[key] = value
	}
}

func EventToRequest(url string, in event.Event) (*http.Request, error) {
	switch in.Mode {
	case event.StructuredMode:
		return structuredEventToRequest(url, in)
	case event.DefaultMode, event.BinaryMode:
		return binaryEventToRequest(url, in)
	}
	return nil, fmt.Errorf("unknown content mode: %q", in.Mode)
}

func structuredEventToRequest(url string, event event.Event) (*http.Request, error) {
	env := make(map[string]interface{})

	// CloudEvents attributes.
	addStructured(env, "specversion", event.Attributes.SpecVersion)
	addStructured(env, "type", event.Attributes.Type)
	addStructured(env, "time", event.Attributes.Time)
	addStructured(env, "id", event.Attributes.ID)
	addStructured(env, "source", event.Attributes.Source)
	addStructured(env, "subject", event.Attributes.Subject)
	addStructured(env, "schemaurl", event.Attributes.SchemaURL)
	addStructured(env, "datacontenttype", event.Attributes.DataContentType)
	addStructured(env, "datacontentencoding", event.Attributes.DataContentEncoding)

	// CloudEvents attribute extensions.
	for k, v := range event.Attributes.Extensions {
		addStructured(env, k, v)
	}

	// TODO: based on datacontenttype, we should parse data and then set the result in the envelope.
	if len(event.Data) > 0 {
		data := json.RawMessage{}
		if err := json.Unmarshal([]byte(event.Data), &data); err != nil {
			return nil, err
		}
		env["data"] = data
	}

	// To JSON.
	body, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Transport extensions.
	hasContentType := false
	for k, v := range event.TransportExtensions {
		if strings.EqualFold(v, "Content-Type") {
			hasContentType = true
		}
		addHeader(req, k, v)
	}

	if !hasContentType {
		addHeader(req, "Content-Type", "application/cloudevents+json; charset=UTF-8")
	}

	return req, nil
}

func binaryEventToRequest(url string, event event.Event) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(event.Data)))
	if err != nil {
		return nil, err
	}

	// CloudEvents attributes.
	addHeader(req, "ce-specversion", event.Attributes.SpecVersion)
	addHeader(req, "ce-type", event.Attributes.Type)
	addHeader(req, "ce-time", event.Attributes.Time)
	addHeader(req, "ce-id", event.Attributes.ID)
	addHeader(req, "ce-source", event.Attributes.Source)
	addHeader(req, "ce-subject", event.Attributes.Subject)
	addHeader(req, "ce-schemaurl", event.Attributes.SchemaURL)
	addHeader(req, "Content-Type", event.Attributes.DataContentType)
	addHeader(req, "ce-datacontentencoding", event.Attributes.DataContentEncoding)

	// CloudEvents attribute extensions.
	for k, v := range event.Attributes.Extensions {
		addHeader(req, "ce-"+k, v)
	}

	// Transport extensions.
	for k, v := range event.TransportExtensions {
		addHeader(req, k, v)
	}

	return req, nil
}

func RequestToEvent(req *http.Request) (*event.Event, error) {
	if strings.HasPrefix(req.Header.Get("Content-Type"), "application/cloudevents+json") {
		req.Header.Del("Content-Type")
		return structuredRequestToEvent(req)
	}
	return binaryRequestToEvent(req)
}

func structuredRequestToEvent(req *http.Request) (*event.Event, error) {
	out := &event.Event{
		Mode: event.StructuredMode,
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = body

	env := make(map[string]json.RawMessage)
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}

	insert := func(key string, into *string) {
		if _, found := env[key]; found {
			if err := json.Unmarshal(env[key], into); err != nil {
				*into = err.Error()
			}
			delete(env, key)
		}
	}

	// CloudEvents attributes.
	insert("specversion", &out.Attributes.SpecVersion)
	insert("type", &out.Attributes.Type)
	insert("time", &out.Attributes.Time)
	insert("id", &out.Attributes.ID)
	insert("source", &out.Attributes.Source)
	insert("subject", &out.Attributes.Subject)
	insert("schemaurl", &out.Attributes.SchemaURL)
	insert("datacontenttype", &out.Attributes.DataContentType)
	insert("datacontentencoding", &out.Attributes.DataContentEncoding)

	// CloudEvents Data.
	if _, found := env["data"]; found {
		out.Data = string(env["data"]) + "\n"
		delete(env, "data")
	}

	// CloudEvents attribute extensions.
	out.Attributes.Extensions = make(map[string]string)
	for key, b := range env {
		var into string
		if err := json.Unmarshal(b, &into); err != nil {
			into = err.Error()
		}
		out.Attributes.Extensions[key] = into
		delete(env, key)
	}

	// Transport extensions.
	out.TransportExtensions = make(map[string]string)
	for k := range req.Header {
		if k == "Accept-Encoding" || k == "Content-Length" {
			continue
		}
		out.TransportExtensions[k] = req.Header.Get(k)
		req.Header.Del(k)
	}

	return out, nil
}

func binaryRequestToEvent(req *http.Request) (*event.Event, error) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = body

	out := &event.Event{
		Mode: event.BinaryMode,
		Data: string(body),
	}

	// CloudEvents attributes.
	out.Attributes.SpecVersion = req.Header.Get("ce-specversion")
	req.Header.Del("ce-specversion")
	out.Attributes.Type = req.Header.Get("ce-type")
	req.Header.Del("ce-type")
	out.Attributes.Time = req.Header.Get("ce-time")
	req.Header.Del("ce-time")
	out.Attributes.ID = req.Header.Get("ce-id")
	req.Header.Del("ce-id")
	out.Attributes.Source = req.Header.Get("ce-source")
	req.Header.Del("ce-source")
	out.Attributes.Subject = req.Header.Get("ce-subject")
	req.Header.Del("ce-subject")
	out.Attributes.SchemaURL = req.Header.Get("ce-schemaurl")
	req.Header.Del("ce-schemaurl")
	out.Attributes.DataContentType = req.Header.Get("Content-Type")
	req.Header.Del("Content-Type")
	out.Attributes.DataContentEncoding = req.Header.Get("ce-datacontentencoding")
	req.Header.Del("ce-datacontentencoding")

	// CloudEvents attribute extensions.
	out.Attributes.Extensions = make(map[string]string)
	for k := range req.Header {
		if strings.HasPrefix(strings.ToLower(k), "ce-") {
			out.Attributes.Extensions[k[len("ce-"):]] = req.Header.Get(k)
			req.Header.Del(k)
		}
	}

	// Transport extensions.
	out.TransportExtensions = make(map[string]string)
	for k := range req.Header {
		if k == "Accept-Encoding" || k == "Content-Length" {
			continue
		}
		out.TransportExtensions[k] = req.Header.Get(k)
		req.Header.Del(k)
	}

	return out, nil
}

func Do(req *http.Request, hook ResultsFn) error {
	resp, err := http.DefaultClient.Do(req)

	if hook != nil {
		// Non-blocking.
		go hook(req, resp, err)
	}

	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("expected 200 level response, got %s", resp.Status)
	}

	// TODO might want something from resp.
	return nil
}
