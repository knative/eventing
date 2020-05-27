package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/event"
)

// AssertEventContextEquals asserts that two event.Event contexts are equals
func AssertEventContextEquals(t testing.TB, want event.EventContext, have event.EventContext) {
	wantVersion := spec.VS.Version(want.GetSpecVersion())
	require.NotNil(t, wantVersion)
	haveVersion := spec.VS.Version(have.GetSpecVersion())
	require.NotNil(t, haveVersion)
	require.Equal(t, wantVersion, haveVersion)

	for _, a := range wantVersion.Attributes() {
		require.Equal(t, a.Get(want), a.Get(have), "Attribute %s does not match: %v != %v", a.PrefixedName(), a.Get(want), a.Get(have))
	}

	require.Equal(t, want.GetExtensions(), have.GetExtensions(), "Extensions")
}

// AssertEventEquals asserts that two event.Event are equals
func AssertEventEquals(t testing.TB, want event.Event, have event.Event) {
	AssertEventContextEquals(t, want.Context, have.Context)
	wantPayload := want.Data()
	havePayload := have.Data()
	assert.Equal(t, wantPayload, havePayload)
}
