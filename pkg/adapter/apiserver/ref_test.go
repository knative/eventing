package apiserver

import (
	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestRefAddEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addEvent(simplePod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceAddRefEventType)
}

func TestRefUpdateEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.updateEvent(nil, simplePod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestRefDeleteEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.deleteEvent(simplePod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceDeleteRefEventType)
}

func TestRefAddEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addEvent(nil)
	validateNotSent(t, ce, v1alpha1.ApiServerSourceAddRefEventType)
}

func TestRefUpdateEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.updateEvent(nil, nil)
	validateNotSent(t, ce, v1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestRefDeleteEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.deleteEvent(nil)
	validateNotSent(t, ce, v1alpha1.ApiServerSourceDeleteRefEventType)
}

func TestRefAddEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.addEvent(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceAddRefEventType)
}

func TestRefUpdateEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.updateEvent(nil, simpleOwnedPod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestRefDeleteEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.deleteEvent(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceDeleteRefEventType)
}
