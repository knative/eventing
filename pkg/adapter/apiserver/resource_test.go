package apiserver

import (
	"github.com/knative/eventing/pkg/adapter/apiserver/events"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestResourceAddEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Add(simplePod("unit", "test"))
	validateSent(t, ce, events.AddEventType)
}

func TestResourceUpdateEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Update(simplePod("unit", "test"))
	validateSent(t, ce, events.UpdateEventType)
}

func TestResourceDeleteEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Delete(simplePod("unit", "test"))
	validateSent(t, ce, events.DeleteEventType)
}

func TestResourceAddEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Add(nil)
	validateNotSent(t, ce, events.AddEventType)
}

func TestResourceUpdateEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Update(nil)
	validateNotSent(t, ce, events.UpdateEventType)
}

func TestResourceDeleteEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Delete(nil)
	validateNotSent(t, ce, events.DeleteEventType)
}

func TestResourceCoverageHacks(t *testing.T) {
	d, _ := makeResourceAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{}) // for coverage.
}

// HACKHACKHACK For test coverage.
func TestResourceStub(t *testing.T) {
	d, _ := makeResourceAndTestingClient()

	d.List()
	d.ListKeys()
	d.Get(nil)
	d.GetByKey("")
	d.Replace(nil, "")
	d.Resync()
}
