package apiserver

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

func TestRefAddEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceAddRefEventType)
	validateMetric(t, d.reporter, 1)
}

func TestRefUpdateEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
	validateMetric(t, d.reporter, 1)
}

func TestRefDeleteEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(simplePod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
	validateMetric(t, d.reporter, 1)
}

func TestRefAddEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceAddRefEventType)
	validateMetric(t, d.reporter, 0)
}

func TestRefUpdateEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
	validateMetric(t, d.reporter, 0)
}

func TestRefDeleteEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(nil)
	validateNotSent(t, ce, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
	validateMetric(t, d.reporter, 0)
}

func TestRefAddEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.Add(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceAddRefEventType)
	validateMetric(t, d.reporter, 1)
}

func TestRefUpdateEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.Update(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
	validateMetric(t, d.reporter, 1)
}

func TestRefDeleteEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.Delete(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
	validateMetric(t, d.reporter, 1)
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
