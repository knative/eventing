package apiserver

import (
	"testing"

	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestRefAddEventAsController(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.addControllerWatch(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	})
	d.Add(simpleOwnedPod("unit", "test"))
	validateSent(t, ce, sourcesv1alpha1.ApiServerSourceAddRefEventType)
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
