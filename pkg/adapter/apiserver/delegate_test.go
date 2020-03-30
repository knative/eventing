package apiserver

import (
	"testing"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

func TestResourceAddEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Add(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceAddEventType)
}

func TestResourceUpdateEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Update(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceUpdateEventType)
}

func TestResourceDeleteEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Delete(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceDeleteEventType)
}

func TestResourceAddEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Add(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceAddEventType)
}

func TestResourceUpdateEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Update(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceUpdateEventType)
}

func TestResourceDeleteEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Delete(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceDeleteEventType)
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
