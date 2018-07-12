/*
Copyright 2018 The Knative Authors

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

package event_test

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/knative/eventing/pkg/event"
)

func TestHandlerTypeErrors(t *testing.T) {
	for _, test := range []struct {
		name  string
		param interface{}
		err   string
	}{
		{
			name:  "non-func",
			param: 5,
			err:   "did not receive a func",
		},
		{
			name:  "wrong param count",
			param: func() {},
			err:   "wrong parameter count",
		},
		{
			name:  "context by value",
			param: func(map[string]interface{}, event.Context) error { return nil },
			err:   "cannot convert", /* <type name> to event.Context */
		},
		{
			name:  "wrong return count",
			param: func(map[string]interface{}, *event.Context) (interface{}, error) { return nil, nil },
			err:   "wrong output count",
		},
		{
			name:  "invalid return type",
			param: func(map[string]interface{}, *event.Context) interface{} { return nil },
			err:   "cannot convert", /* <type name> to error */
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("Did not panic")
				}
				str := r.(string)
				if !strings.Contains(str, test.err) {
					t.Fatalf("Got panic %q; which should have contained %q", str, test.err)
				}
			}()

			event.Handler(test.param)
		})
	}
}

func TestUntypedHandling(t *testing.T) {
	expectedData := map[string]interface{}{
		"hello": "world!",
	}
	expectedContext := &event.Context{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		Extensions:         map[string]interface{}{},
	}
	handler := event.Handler(func(data map[string]interface{}, ctx *event.Context) error {
		if !reflect.DeepEqual(expectedData, data) {
			t.Fatalf("Did not get expected data: wanted=%s; got=%s",
				spew.Sdump(expectedData), spew.Sdump(data))
		}
		if !reflect.DeepEqual(expectedContext, ctx) {
			t.Fatalf("Did not get expected contex: wanted=%s; got=%s",
				spew.Sdump(expectedContext), spew.Sdump(ctx))
		}
		return nil
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, err := event.NewRequest(srv.URL, expectedData, *expectedContext)
	if err != nil {
		t.Fatal("Failed to marshal request ", err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal("Failed to send request")
	}
	if res.StatusCode/100 != 2 {
		t.Fatal("Got non-successful response: ", res.StatusCode)
	}
}

func TestTypedHandling(t *testing.T) {
	type Data struct {
		Message string
	}
	expectedData := Data{Message: "Hello, world!"}
	expectedContext := &event.Context{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		Extensions:         map[string]interface{}{},
	}
	handler := event.Handler(func(data Data, ctx *event.Context) error {
		if !reflect.DeepEqual(expectedData, data) {
			t.Fatalf("Did not get expected data: wanted=%s; got=%s",
				spew.Sdump(expectedData), spew.Sdump(data))
		}
		if !reflect.DeepEqual(expectedContext, ctx) {
			t.Fatalf("Did not get expected contex: wanted=%s; got=%s",
				spew.Sdump(expectedContext), spew.Sdump(ctx))
		}
		return nil
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, err := event.NewRequest(srv.URL, expectedData, *expectedContext)
	if err != nil {
		t.Fatal("Failed to marshal request ", err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal("Failed to send request")
	}
	if res.StatusCode/100 != 2 {
		t.Fatal("Got non-successful response: ", res.StatusCode)
	}
}

func TestPointerHandling(t *testing.T) {
	type Data struct {
		Message string
	}
	expectedData := &Data{Message: "Hello, world!"}
	expectedContext := &event.Context{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		Extensions:         map[string]interface{}{},
	}
	handler := event.Handler(func(data *Data, ctx *event.Context) error {
		if !reflect.DeepEqual(expectedData, data) {
			t.Fatalf("Did not get expected data: wanted=%s; got=%s",
				spew.Sdump(expectedData), spew.Sdump(data))
		}
		if !reflect.DeepEqual(expectedContext, ctx) {
			t.Fatalf("Did not get expected contex: wanted=%s; got=%s",
				spew.Sdump(expectedContext), spew.Sdump(ctx))
		}
		return nil
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, err := event.NewRequest(srv.URL, expectedData, *expectedContext)
	if err != nil {
		t.Fatal("Failed to marshal request ", err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal("Failed to send request")
	}
	if res.StatusCode/100 != 2 {
		t.Fatal("Got non-successful response: ", res.StatusCode)
	}
}

func TestMux(t *testing.T) {
	type TypeA struct {
		Greeting string
	}
	type TypeB struct {
		Farewell string
	}

	eventA := TypeA{
		Greeting: "Hello, world!",
	}
	eventB := TypeB{
		Farewell: "Hasta la vista",
	}

	contextA := &event.Context{
		EventID:     "1234",
		EventType:   "org.A.test",
		Source:      "test:TestMux",
		ContentType: "application/json",
		Extensions:  map[string]interface{}{},
	}
	contextB := &event.Context{
		EventID:     "5678",
		EventType:   "org.B.test",
		Source:      "test:TestMux",
		ContentType: "application/json",
		Extensions:  map[string]interface{}{},
	}
	sawA, sawB := false, false

	mux := event.NewMux()
	mux.Handle("org.A.test", func(data TypeA, context *event.Context) error {
		sawA = true
		if !reflect.DeepEqual(eventA, data) {
			t.Fatalf("Got wrong data for event A; wanted=%s; got=%s", eventA, data)
		}
		if !reflect.DeepEqual(contextA, context) {
			t.Fatalf("Got wrong context for event A; wanted=%s; got=%s", contextA, context)
		}
		return nil
	})
	mux.Handle("org.B.test", func(data TypeB, context *event.Context) error {
		sawB = true
		if !reflect.DeepEqual(eventB, data) {
			t.Fatalf("Got wrong data for event A; wanted=%s; got=%s", eventB, data)
		}
		if !reflect.DeepEqual(contextB, context) {
			t.Fatalf("Got wrong context for event A; wanted=%s; got=%s", contextB, context)
		}
		return nil
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, err := event.NewRequest(srv.URL, eventA, *contextA)
	if err != nil {
		t.Fatal("Failed to marshal request for eventA", err)
	}
	if _, err := srv.Client().Do(req); err != nil {
		t.Fatal("Failed to send eventA", err)
	}
	req, err = event.NewRequest(srv.URL, eventB, *contextB)
	if err != nil {
		t.Fatal("Failed to marshal request for eventB", err)
	}
	if _, err := srv.Client().Do(req); err != nil {
		t.Fatal("Failed to send eventB", err)
	}

	if !sawA {
		t.Fatal("Handler for eventA never called")
	}
	if !sawB {
		t.Fatal("Hanlder for eventB never called")
	}
}
