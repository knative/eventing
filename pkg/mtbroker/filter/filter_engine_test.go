package filter

import (
	"testing"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/require"
)

func TestParseFilterExpr(t *testing.T) {
	program, err := ParseFilterExpr("/w3schools/i.test(xxx)")
	require.NoError(t, err)
	vm := otto.New()
	vm.Set("xxx", "w3schools")
	val, err := vm.Run(program)
	require.NoError(t, err)
	b, err := val.ToBoolean()
	require.NoError(t, err)
	require.True(t, b)
}

func TestRunFilterTrue(t *testing.T) {
	event := test.FullEvent()

	program, err := ParseFilterExpr("event.id() == \"" + event.ID() + "\"")
	require.NoError(t, err)
	res, err := RunFilter(event, program)
	require.NoError(t, err)
	require.True(t, res)
}

func TestRunFilterFalse(t *testing.T) {
	event := test.FullEvent()

	program, err := ParseFilterExpr("event.id() != \"" + event.ID() + "\"")
	require.NoError(t, err)
	res, err := RunFilter(event, program)
	require.NoError(t, err)
	require.False(t, res)
}
