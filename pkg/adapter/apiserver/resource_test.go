package apiserver

import (
	"github.com/knative/eventing/pkg/adapter/apiserver/events"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestResourceAddEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.addEvent(simplePod("unit", "test"))
	validateSent(t, ce, events.AddEventType)
}

func TestResourceUpdateEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.updateEvent(nil, simplePod("unit", "test"))
	validateSent(t, ce, events.UpdateEventType)
}

func TestResourceDeleteEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.deleteEvent(simplePod("unit", "test"))
	validateSent(t, ce, events.DeleteEventType)
}

func TestResourceAddEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.addEvent(nil)
	validateNotSent(t, ce, events.AddEventType)
}

func TestResourceUpdateEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.updateEvent(nil, nil)
	validateNotSent(t, ce, events.UpdateEventType)
}

func TestResourceDeleteEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.deleteEvent(nil)
	validateNotSent(t, ce, events.DeleteEventType)
}

func TestResourceCoverageHacks(t *testing.T) {
	d, _ := makeResourceAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{}) // for coverage.
}
