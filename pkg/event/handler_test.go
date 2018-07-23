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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

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
			h := event.Handler(test.param)
			err, ok := h.(error)
			if !ok {
				t.Fatalf("Expected Handler() to fail with %v, passed", test.err)
			}
			if !strings.Contains(err.Error(), test.err) {
				t.Errorf("Expected Handler() to fail. want %q, got %v", test.err, err)
			}

			// Attempt to call the returned Handler to verify it fails.
			srv := httptest.NewServer(h)
			defer srv.Close()
			type E struct{}
			req, err := event.NewRequest(srv.URL, E{}, event.EventContext{
				EventID: "1", EventType: "a", Source: "one"})
			if err != nil {
				t.Errorf("Couldn't construct request: %v", err)
			}
			if resp, err := srv.Client().Do(req); err != nil {
				t.Errorf("Failed to Post event: %v", resp)
			} else if resp.StatusCode != http.StatusNotImplemented {
				t.Errorf("Expected error status. got %d, got %d", resp.StatusCode, http.StatusNotImplemented)
			}
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
			if err, ok := event.Handler(test.f).(error); ok {
				t.Errorf("%q failed: %v", test.name, err)
			}
		})
	}
}

func TestParameterMarsahlling(t *testing.T) {
	type Data struct {
		Message string
	}
	expectedData := Data{Message: "Hello, world!"}
	expectedContext := &event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		EventTime:          time.Now().UTC(),
		ContentType:        "application/json",
		Extensions:         map[string]interface{}{},
	}
	var wasCalled = false
	for _, marshaller := range []struct {
		name string
		val  event.HTTPMarshaller
	}{
		{
			name: "structured",
			val:  event.Structured,
		}, {
			name: "binary",
			val:  event.Binary,
		},
	} {
		for _, test := range []struct {
			name      string
			generator func(t *testing.T) http.Handler
		}{
			{
				name: "no parameters",
				generator: func(t *testing.T) http.Handler {
					return event.Handler(func() {
						wasCalled = true
					})
				},
			},
			{
				name: "one parameter",
				generator: func(t *testing.T) http.Handler {
					return event.Handler(func(ctx context.Context) {
						eventContext := event.FromContext(ctx)
						if !reflect.DeepEqual(expectedContext, eventContext) {
							t.Fatalf("Did not get expected context; wanted=%s; got=%s", spew.Sdump(expectedContext), spew.Sdump(eventContext))
						}
						wasCalled = true
					})
				},
			}, {
				name: "two parameters (struct type)",
				generator: func(t *testing.T) http.Handler {
					return event.Handler(func(ctx context.Context, data Data) {
						eventContext := event.FromContext(ctx)
						if !reflect.DeepEqual(expectedContext, eventContext) {
							t.Fatalf("Did not get expected context; wanted=%s; got=%s", spew.Sdump(expectedContext), spew.Sdump(eventContext))
						}
						if !reflect.DeepEqual(expectedData, data) {
							t.Fatalf("Did not get expected data; wanted=%s; got=%s", spew.Sdump(expectedData), spew.Sdump(data))
						}
						wasCalled = true
					})
				},
			}, {
				name: "two parameters (pointer type)",
				generator: func(t *testing.T) http.Handler {
					return event.Handler(func(ctx context.Context, data *Data) {
						eventContext := event.FromContext(ctx)
						if !reflect.DeepEqual(expectedContext, eventContext) {
							t.Fatalf("Did not get expected context; wanted=%s; got=%s", spew.Sdump(expectedContext), spew.Sdump(eventContext))
						}
						if !reflect.DeepEqual(expectedData, *data) {
							t.Fatalf("Did not get expected data; wanted=%s; got=%s", spew.Sdump(&expectedData), spew.Sdump(data))
						}
						wasCalled = true
					})
				},
			}, {
				name: "two parameters (untyped)",
				generator: func(t *testing.T) http.Handler {
					return event.Handler(func(ctx context.Context, data map[string]interface{}) {
						eventContext := event.FromContext(ctx)
						if !reflect.DeepEqual(expectedContext, eventContext) {
							t.Fatalf("Did not get expected context; wanted=%s; got=%s", spew.Sdump(expectedContext), spew.Sdump(eventContext))
						}
						b, err := json.Marshal(expectedData)
						if err != nil {
							t.Fatal("Failed to serialize expected data", err)
						}
						var expectedUntyped map[string]interface{}
						err = json.Unmarshal(b, &expectedUntyped)
						if err != nil {
							t.Fatal("Failed to deserialize expected data", err)
						}
						if !reflect.DeepEqual(expectedUntyped, data) {
							t.Fatalf("Did not get expected data; wanted=%s; got=%s", spew.Sdump(expectedUntyped), spew.Sdump(data))
						}
						wasCalled = true
					})
				},
			},
		} {
			t.Run(fmt.Sprintf("%s: %s", marshaller.name, test.name), func(t *testing.T) {
				wasCalled = false
				handler := test.generator(t)
				if err, ok := handler.(error); ok {
					t.Errorf("Handler() failed: %v", err)
					return
				}
				srv := httptest.NewServer(handler)
				defer srv.Close()
				req, err := marshaller.val.NewRequest(srv.URL, expectedData, *expectedContext)
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
				if !wasCalled {
					t.Fatal("Handler was never called")
				}
			})
		}
	}
}

func TestReturnTypeRendering(t *testing.T) {
	eventData := map[string]interface{}{
		"unused": "data",
	}
	type RetVal struct {
		ID interface{}
	}
	eventContext := event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventID:            "1234",
		Source:             "tests:TestUndtypedHandling",
		EventType:          "dev.eventing.test",
		Extensions:         map[string]interface{}{},
	}
	for _, test := range []struct {
		name             string
		expectedStatus   int
		expectedResponse string
		handler          http.Handler
	}{
		{
			name:           "no return",
			expectedStatus: http.StatusNoContent,
			handler:        event.Handler(func() {}),
		}, {
			name:           "nil error return",
			expectedStatus: http.StatusNoContent,
			handler: event.Handler(func() error {
				return nil
			}),
		}, {
			name:           "non-nil error return (one return type)",
			expectedStatus: http.StatusInternalServerError,
			handler: event.Handler(func() error {
				return errors.New("Some error")
			}),
			expectedResponse: "Internal server error",
		}, {
			name:           "successful return",
			expectedStatus: http.StatusOK,
			handler: event.Handler(func() (map[string]interface{}, error) {
				return map[string]interface{}{"hello": "world"}, nil
			}),
			expectedResponse: `{"hello":"world"}`,
		}, {
			name:           "non-nil error return (two return types)",
			expectedStatus: http.StatusInternalServerError,
			handler: event.Handler(func() (map[string]interface{}, error) {
				return map[string]interface{}{"hello": "world"}, errors.New("Errors take precedence")
			}),
			expectedResponse: "Internal server error",
		},
		{
			name:           "non-nil content return",
			expectedStatus: http.StatusOK,
			handler: event.Handler(func() (map[string]interface{}, error) {
				return map[string]interface{}{"hello": "world"}, nil
			}),
			expectedResponse: `{"hello":"world"}`,
		},
		{
			name:           "bad JSON content",
			expectedStatus: http.StatusInternalServerError,
			handler: event.Handler(func() (RetVal, error) {
				return RetVal{ID: func() {}}, nil
			}),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err, ok := test.handler.(error); ok {
				t.Errorf("Handler() failed: %v", err)
				return
			}
			srv := httptest.NewServer(test.handler)
			defer srv.Close()
			req, err := event.NewRequest(srv.URL, eventData, eventContext)
			if err != nil {
				t.Fatal("Failed to marshal request ", err)
			}
			res, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal("Failed to send request")
			}
			defer res.Body.Close()
			if test.expectedStatus != res.StatusCode {
				t.Fatalf("Wrong status code from event handler; wanted=%d; got=%d", test.expectedStatus, res.StatusCode)
			}
			if test.expectedResponse != "" {
				b, err := ioutil.ReadAll(res.Body)
				if err != nil {
					t.Fatal("Failed to read response body:", err)
				}
				resBody := string(b)
				if test.expectedResponse != resBody {
					t.Fatalf("Got unexpected respnose string; wanted=%q; got=%q", test.expectedResponse, resBody)
				}
			}
		})
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
	err := mux.Handle("org.A.test", func(ctx context.Context, data TypeA) error {
		sawA = true
		context := event.FromContext(ctx)
		if !reflect.DeepEqual(eventA, data) {
			t.Fatalf("Got wrong data for event A; wanted=%s; got=%s", eventA, data)
		}
		if !reflect.DeepEqual(contextA, context) {
			t.Fatalf("Got wrong context for event A; wanted=%s; got=%s", contextA, context)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("mux.Handle('org.A.test') failed: %v", err)
	}
	err = mux.Handle("org.B.test", func(ctx context.Context, data TypeB) error {
		sawB = true
		context := event.FromContext(ctx)
		if !reflect.DeepEqual(eventB, data) {
			t.Fatalf("Got wrong data for event A; wanted=%s; got=%s", eventB, data)
		}
		if !reflect.DeepEqual(contextB, context) {
			t.Fatalf("Got wrong context for event A; wanted=%s; got=%s", contextB, context)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("mux.Handle('org.B.test') failed: %v", err)
	}

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
