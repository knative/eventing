package apiserver

import (
	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestResourceAddEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.addEvent(simplePod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceAddEventType)
}

func TestResourceUpdateEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.updateEvent(nil, simplePod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceUpdateEventType)
}

func TestResourceDeleteEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.deleteEvent(simplePod("unit", "test"))
	validateSent(t, ce, v1alpha1.ApiServerSourceDeleteEventType)
}

func TestResourceAddEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.addEvent(nil)
	validateNotSent(t, ce, v1alpha1.ApiServerSourceAddEventType)
}

func TestResourceUpdateEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.updateEvent(nil, nil)
	validateNotSent(t, ce, v1alpha1.ApiServerSourceUpdateEventType)
}

func TestResourceDeleteEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.deleteEvent(nil)
	validateNotSent(t, ce, v1alpha1.ApiServerSourceDeleteEventType)
}

func TestResourceCoverageHacks(t *testing.T) {
	d, _ := makeResourceAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{}) // for coverage.
}
