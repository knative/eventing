package test

import (
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
)

// AssertEvent is a "matcher like" assertion method to test the properties of an event
func AssertEvent(t testing.TB, have event.Event, matchers ...EventMatcher) {
	err := AllOf(matchers...)(have)
	if err != nil {
		t.Fatalf("Error while matching event: %s", err.Error())
	}
}

// AssertEventContextEquals asserts that two event.Event contexts are equals
func AssertEventContextEquals(t testing.TB, want event.EventContext, have event.EventContext) {
	if err := IsContextEqualTo(want)(event.Event{Context: have}); err != nil {
		t.Fatalf("Error while matching event context: %s", err.Error())
	}
}

// AssertEventEquals asserts that two event.Event are equals
func AssertEventEquals(t testing.TB, want event.Event, have event.Event) {
	if err := IsEqualTo(want)(have); err != nil {
		t.Fatalf("Error while matching event: %s", err.Error())
	}
}
