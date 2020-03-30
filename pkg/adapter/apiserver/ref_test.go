package apiserver

import (
	"testing"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

func TestRefAddEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceAddRefEventType)
}

func TestRefUpdateEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestRefDeleteEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)

}

func TestRefAddEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceAddRefEventType)
}

func TestRefUpdateEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestRefDeleteEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
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
