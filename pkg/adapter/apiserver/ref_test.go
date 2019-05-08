package apiserver

import (
	"github.com/knative/eventing/pkg/adapter/apiserver/events"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestRefAddEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addEvent(simplePod("unit", "test"))
	validateSent(t, ce, events.AddEventRefType)
}

func TestRefUpdateEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.updateEvent(nil, simplePod("unit", "test"))
	validateSent(t, ce, events.UpdateEventRefType)
}

func TestRefDeleteEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.deleteEvent(simplePod("unit", "test"))
	validateSent(t, ce, events.DeleteEventRefType)
}

func TestRefAddEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addEvent(nil)
	validateNotSent(t, ce, events.AddEventRefType)
}

func TestRefUpdateEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.updateEvent(nil, nil)
	validateNotSent(t, ce, events.UpdateEventRefType)
}

func TestRefDeleteEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.deleteEvent(nil)
	validateNotSent(t, ce, events.DeleteEventRefType)
}

func TestRefAddEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.addEvent(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, events.AddEventRefType)
}

func TestRefUpdateEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.updateEvent(nil, simpleOwnedPod("unit", "test"))
	validateSent(t, ce, events.UpdateEventRefType)
}

func TestRefDeleteEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.deleteEvent(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, events.DeleteEventRefType)
}
