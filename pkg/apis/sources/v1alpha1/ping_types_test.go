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

package v1alpha1

import "testing"

func TestPingSource_GetGroupVersionKind(t *testing.T) {
	src := PingSource{}
	gvk := src.GetGroupVersionKind()

	if gvk.Kind != "PingSource" {
		t.Errorf("Should be PingSource.")
	}
}

func TestPingSource_PingSourceSource(t *testing.T) {
	cePingSource := PingSourceSource("ns1", "job1")

	if cePingSource != "/apis/v1/namespaces/ns1/pingsources/job1" {
		t.Errorf("Should be '/apis/v1/namespaces/ns1/pingsources/job1'")
	}
}
