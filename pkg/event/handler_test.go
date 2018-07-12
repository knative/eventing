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
	"context"
	"io"
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
			err:   "Must pass a function to handle events",
		},
		{
			name:  "wrong param count",
			param: func(context.Context, interface{}, interface{}) {},
			err:   "Expected a function taking either no parameters, a context.Context, or (context.Context, any); function has too many parameters (3)",
		},
		{
			name:  "wrong first parameter type",
			param: func(int) {},
			err:   "Expected a function taking either no parameters, a context.Context, or (context.Context, any); cannot convert parameter 0 from int to context.Context",
		},
		{
			name:  "wrong return count",
			param: func() (interface{}, error, interface{}) { return nil, nil, nil },
			err:   "Expected a function returning either nothing, an error, or (any, error); function has too many return types (3)",
		},
		{
			name:  "invalid return type",
			param: func() interface{} { return nil },
			err:   "Expected a function returning either nothing, an error, or (any, error); cannot convert return type 0 from interface {} to error",
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

func TestHandlerValidTypes(t *testing.T) {
	for _, test := range []struct {
		name string
		f    interface{}
	}{
		{
			name: "no in, no out",
			f:    func() {},
		}, {
			name: "one in, no out",
			f:    func(context.Context) {},
		}, {
			name: "interface in, no out",
			f:    func(context.Context, io.Reader) {},
		}, {
			name: "value-type in, no out",
			f:    func(context.Context, int) {},
		}, {
			name: "pointer-type in, no out",
			f:    func(context.Context, *int) {},
		}, {
			name: "no in, one out",
			f:    func() error { return nil },
		}, {
			name: "one in, one out",
			f:    func(context.Context) error { return nil },
		}, {
			name: "two in, one out",
			f:    func(context.Context, string) error { return nil },
		}, {
			name: "no in, two out",
			f:    func() (string, error) { return "", nil },
		}, {
			name: "one in, two out",
			f:    func(context.Context) (map[string]interface{}, error) { return nil, nil },
		}, {
			name: "two in, two out",
			f:    func(context.Context, io.Reader) (interface{}, error) { return nil, nil },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			event.Handler(test.f)
		})
	}
}

func TestUntypedHandling(t *testing.T) {
	expectedData := map[string]interface{}{
		"hello": "world!",
	}
	expectedContext := &event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		Extensions:         map[string]interface{}{},
	}
	handler := event.Handler(func(data map[string]interface{}, ctx *event.EventContext) error {
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
	expectedContext := &event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		Extensions:         map[string]interface{}{},
	}
	handler := event.Handler(func(data Data, ctx *event.EventContext) error {
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
	expectedContext := &event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		Extensions:         map[string]interface{}{},
	}
	handler := event.Handler(func(data *Data, ctx *event.EventContext) error {
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

	contextA := &event.EventContext{
		EventID:     "1234",
		EventType:   "org.A.test",
		Source:      "test:TestMux",
		ContentType: "application/json",
		Extensions:  map[string]interface{}{},
	}
	contextB := &event.EventContext{
		EventID:     "5678",
		EventType:   "org.B.test",
		Source:      "test:TestMux",
		ContentType: "application/json",
		Extensions:  map[string]interface{}{},
	}
	sawA, sawB := false, false

	mux := event.NewMux()
	mux.Handle("org.A.test", func(data TypeA, context *event.EventContext) error {
		sawA = true
		if !reflect.DeepEqual(eventA, data) {
			t.Fatalf("Got wrong data for event A; wanted=%s; got=%s", eventA, data)
		}
		if !reflect.DeepEqual(contextA, context) {
			t.Fatalf("Got wrong context for event A; wanted=%s; got=%s", contextA, context)
		}
		return nil
	})
	mux.Handle("org.B.test", func(data TypeB, context *event.EventContext) error {
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
