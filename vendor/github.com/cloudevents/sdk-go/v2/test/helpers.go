package test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/types"
)

// WithoutExtensions returns a copy of events with no Extensions.
// Use for testing where extensions are not supported.
func WithoutExtensions(events []event.Event) []event.Event {
	result := make([]event.Event, len(events))
	for i, e := range events {
		result[i] = e
		result[i].Context = e.Context.Clone()
		ctx := reflect.ValueOf(result[i].Context).Elem()
		ext := ctx.FieldByName("Extensions")
		ext.Set(reflect.Zero(ext.Type()))
	}
	return result
}

// MustJSON marshals the event.Event to JSON structured representation or panics
func MustJSON(t testing.TB, e event.Event) []byte {
	b, err := format.JSON.Marshal(&e)
	require.NoError(t, err)
	return b
}

// MustToEvent converts a Message to event.Event
func MustToEvent(t testing.TB, ctx context.Context, m binding.Message) event.Event {
	e, err := binding.ToEvent(ctx, m)
	require.NoError(t, err)
	return *e
}

// ConvertEventExtensionsToString returns a copy of the event.Event where all extensions are converted to strings. Fails the test if conversion fails
func ConvertEventExtensionsToString(t testing.TB, e event.Event) event.Event {
	out := e.Clone()
	for k, v := range e.Extensions() {
		var vParsed interface{}
		var err error

		switch v := v.(type) {
		case json.RawMessage:
			err = json.Unmarshal(v, &vParsed)
			require.NoError(t, err)
		default:
			vParsed, err = types.Format(v)
			require.NoError(t, err)
		}
		out.SetExtension(k, vParsed)
	}
	return out
}

// TestNameOf generates a string test name from x, esp. for ce.Event and ce.Message.
func TestNameOf(x interface{}) string {
	switch x := x.(type) {
	case event.Event:
		b, err := json.Marshal(x)
		if err == nil {
			return fmt.Sprintf("Event%s", b)
		}
	case binding.Message:
		return fmt.Sprintf("Message{%s}", reflect.TypeOf(x).String())
	}
	return fmt.Sprintf("%T(%#v)", x, x)
}

// EachEvent runs f as a test for each event in events
func EachEvent(t *testing.T, events []event.Event, f func(*testing.T, event.Event)) {
	for _, e := range events {
		in := e
		t.Run(TestNameOf(in), func(t *testing.T) { f(t, in) })
	}
}

// EachMessage runs f as a test for each message in messages
func EachMessage(t *testing.T, messages []binding.Message, f func(*testing.T, binding.Message)) {
	for _, m := range messages {
		in := m
		t.Run(TestNameOf(in), func(t *testing.T) { f(t, in) })
	}
}
