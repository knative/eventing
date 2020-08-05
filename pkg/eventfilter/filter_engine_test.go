package eventfilter

import (
	"testing"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestParseFilterExpr(t *testing.T) {
	program, err := ParseFilterExpr("/w3schools/i.test(xxx)")
	require.NoError(t, err)

	vm := goja.New()
	vm.Set("xxx", "w3schools")
	val, err := vm.RunProgram(program)
	require.NoError(t, err)

	b := val.ToBoolean()
	require.True(t, b)
}

func TestParseFilterFailure(t *testing.T) {
	program, err := ParseFilterExpr(`function helloWorld() {}`)
	require.Error(t, err)
	require.Nil(t, program)
}

func TestEventKeys(t *testing.T) {
	event := test.FullEvent()

	program, err := ParseFilterExpr("Object.keys(event)")
	require.NoError(t, err)

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	vm.Set("event", obj)
	val, err := vm.RunProgram(program)
	require.NoError(t, err)

	s := val.Export()
	require.Contains(t, s, "specversion")
	require.Contains(t, s, "id")
	require.Contains(t, s, "type")
	require.Contains(t, s, "source")
	require.Contains(t, s, "subject")
	require.Contains(t, s, "time")
	require.Contains(t, s, "dataschema")
	require.Contains(t, s, "datacontenttype")

	require.Contains(t, s, "exbool")
	require.Contains(t, s, "exint")
	require.Contains(t, s, "exstring")
	require.Contains(t, s, "exbinary")
	require.Contains(t, s, "exurl")
	require.Contains(t, s, "extime")
}

func TestAccessExtension(t *testing.T) {
	event := test.FullEvent()
	event.SetExtension("someint", 10)

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	vm.Set("event", obj)
	val, err := vm.RunString("event.someint")
	require.NoError(t, err)

	s := val.Export()
	require.Equal(t, int64(10), s)
}

func TestDate(t *testing.T) {
	event := test.FullEvent()

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	vm.Set("event", obj)
	val, err := vm.RunString("event.time.getFullYear()")
	require.NoError(t, err)

	s := val.Export()
	require.Equal(t, int64(2020), s)
}

func TestRunFilter(t *testing.T) {
	event := test.FullEvent()

	tests := []struct {
		expression string
		result     bool
	}{{
		expression: "event.id === \"" + event.ID() + "\"",
		result:     true,
	}, {
		expression: "event.id !== \"" + event.ID() + "\"",
		result:     false,
	}, {
		expression: `event.datacontenttype.indexOf("json") != -1 && true`,
		result:     true,
	}, {
		expression: `event.time.getFullYear() == 2020`,
		result:     true,
	}, {
		expression: `event.exint === 42`,
		result:     true,
	}}
	for _, tc := range tests {
		t.Run(tc.expression, func(t *testing.T) {
			program, err := ParseFilterExpr(tc.expression)
			require.NoError(t, err)
			res, err := RunFilter(event, program)
			require.NoError(t, err)
			require.Equal(t, tc.result, res)
		})
	}
}
