/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package apiserver

import (
	"testing"

	sources "knative.dev/eventing/pkg/apis/sources"
)

func TestRefAddEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(simplePod("unit", "test"))
	validateSent(t, ce, sources.ApiServerSourceAddRefEventType)
}

func TestRefUpdateEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(simplePod("unit", "test"))
	validateSent(t, ce, sources.ApiServerSourceUpdateRefEventType)
}

func TestRefDeleteEvent(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(simplePod("unit", "test"))
	validateSent(t, ce, sources.ApiServerSourceDeleteRefEventType)

}

func TestRefAddEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Add(nil)
	validateNotSent(t, ce, sources.ApiServerSourceAddRefEventType)
}

func TestRefUpdateEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Update(nil)
	validateNotSent(t, ce, sources.ApiServerSourceUpdateRefEventType)
}

func TestRefDeleteEventNil(t *testing.T) {
	d, ce := makeRefAndTestingClient()
	d.Delete(nil)
	validateNotSent(t, ce, sources.ApiServerSourceDeleteRefEventType)
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
