package apiserver

import (
	"github.com/knative/eventing/pkg/adapter/apiserver/events"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestRefAddEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(simplePod("unit", "test"))
	validateSent(t, ce, events.AddEventRefType)
}

func TestRefUpdateEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(simplePod("unit", "test"))
	validateSent(t, ce, events.UpdateEventRefType)
}

func TestRefDeleteEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(simplePod("unit", "test"))
	validateSent(t, ce, events.DeleteEventRefType)
}

func TestRefAddEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(nil)
	validateNotSent(t, ce, events.AddEventRefType)
}

func TestRefUpdateEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(nil)
	validateNotSent(t, ce, events.UpdateEventRefType)
}

func TestRefDeleteEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(nil)
	validateNotSent(t, ce, events.DeleteEventRefType)
}

func TestRefAddEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.Add(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, events.AddEventRefType)
}

func TestRefUpdateEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.Update(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, events.UpdateEventRefType)
}

func TestRefDeleteEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.Delete(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, events.DeleteEventRefType)
}

// HACKHACKHACK For test coverage.
func TestRefStub(t *testing.T) {
	d, _ := makeRefAndTestingClient()

	d.List()
	d.ListKeys()
	d.Get(nil)
	d.GetByKey("")
	d.Replace(nil, "")
	d.Resync()
}
