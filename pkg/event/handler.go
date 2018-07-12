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

package event

import (
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

// TODO(inlined): Change function signature to be a more traditional
// func(context.Context, any) (any, error)
// where both in & out param lists can be of size 0, 1, 2
const usage = "event.NewHandler expected a `func(<data>, event.Context) error`"

type handler struct {
	fnValue  reflect.Value
	dataType reflect.Type
}

// Verifies that the inputs to a function have a valid signature; panics otherwise.
// Valid input signatures:
// (any, event.Context)
func assertInParamSignature(fnType reflect.Type) {
	if fnType.NumIn() != 2 {
		panic(usage + "; wrong parameter count")
	}
	if !fnType.In(1).ConvertibleTo(reflect.TypeOf(&Context{})) {
		panic(usage + "; cannot convert " + fnType.In(1).Name() + " to event.Context")
	}
}

// Verifies that the outputs of a function have a valid signature; panics otherwise.
// Valid output signatures:
// (error)
func assertOutParamSignature(fnType reflect.Type) {
	if fnType.NumOut() != 1 {
		panic(fmt.Sprintf("%s; wrong output count. Expected 1, got %d", usage, fnType.NumOut()))
	}
	// We have to use an awkward jump into and out of a pointer to avoid passing a literal
	// nil to reflect, which would lose all type information and assert.
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if !fnType.Out(0).ConvertibleTo(errorType) {
		panic(usage + "; cannot convert " + fnType.Out(0).Name() + " to error")
	}
}

// Verifies that a function has the right number of in and out params and that they are
// of allowed types. If successful, returns the expected in-param type, otherwise panics.
func assertEventHandler(fn interface{}) (dataType reflect.Type) {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic(usage + "; did not receive a func")
	}
	assertInParamSignature(fnType)
	assertOutParamSignature(fnType)

	return fnType.In(0)
}

// Alocates a new instance of type t and returns:
// asPtr is of type t if t is a pointer type and of type &t otherwise (used for unmarshalling)
// asValue is a Value of type t pointing to the same data as asPtr
func allocate(t reflect.Type) (asPtr interface{}, asValue reflect.Value) {
	if t.Kind() == reflect.Ptr {
		reflectPtr := reflect.New(t.Elem())
		asPtr = reflectPtr.Interface()
		asValue = reflectPtr
	} else {
		reflectPtr := reflect.New(t)
		asPtr = reflectPtr.Interface()
		asValue = reflectPtr.Elem()
	}
	return
}

// Accepts the results from a handler functions and translates them to an HTTP response
func respondHTTP(res []reflect.Value, w http.ResponseWriter) {
	errVal := res[0]
	if errVal.IsNil() {
		return
	}

	// Type cast safe due to assertEventHandler()
	err := errVal.Interface().(error)
	glog.Error("Failed to handle event: ", err)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`Internal server error`))
}

// Handler creates an EventHandler that implements http.Handler
// Will panic in case of a type error
// * fn a function of type func(<your data struct>, *event.Context) error
// TODO(inlined): for continuations we'll probably change the return signature to (interface{}, error)
func Handler(fn interface{}) http.Handler {
	return &handler{dataType: assertEventHandler(fn), fnValue: reflect.ValueOf(fn)}
}

// ServeHTTP implements http.Handler
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dataPtr, dataArg := allocate(h.dataType)

	context, err := FromRequest(dataPtr, r)
	if err != nil {
		glog.Warning("Failed to handle request", spew.Sdump(r))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Invalid request`))
		return
	}

	args := []reflect.Value{dataArg, reflect.ValueOf(context)}
	res := h.fnValue.Call(args)
	respondHTTP(res, w)
}

// Mux allows developers to handle logically related groups of
// functionality multiplexed based on the event type.
// TOOD: Consider dropping Mux or figure out how to handle non-JSON encoding.
type Mux map[string]*handler

// NewMux creates a new Mux
func NewMux() Mux {
	return make(map[string]*handler)
}

// Handle adds a new handler for a specific event type
func (m Mux) Handle(eventType string, fn interface{}) {
	m[eventType] = &handler{dataType: assertEventHandler(fn), fnValue: reflect.ValueOf(fn)}
}

// ServeHTTP implements http.Handler
func (m Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rawData io.Reader
	context, err := FromRequest(&rawData, r)
	if err != nil {
		glog.Warning("Failed to handle request", spew.Sdump(r))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Invalid request`))
		return
	}

	h := m[context.EventType]
	if h == nil {
		glog.Warning("Cloud not find handler for event type", context.EventType)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Event type %q is not supported", context.EventType)))
		return
	}

	dataPtr, dataArg := allocate(h.dataType)
	if err := unmarshalEventData(context.ContentType, rawData, dataPtr); err != nil {
		glog.Warning("Failed to parse event data", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Invalid request`))
		return
	}

	args := []reflect.Value{dataArg, reflect.ValueOf(context)}
	res := h.fnValue.Call(args)
	respondHTTP(res, w)
}
