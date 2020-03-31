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

	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

func TestControllerAddEventWithNoController(t *testing.T) {
	c, tc := makeController("v1", "Pod")
	c.Add(simplePod("unit", "test"))
	validateNotSent(t, tc, sourcesv1alpha1.ApiServerSourceAddRefEventType)
}

func TestControllerAddEventWithWrongController(t *testing.T) {
	c, tc := makeController("v1", "Pod")
	c.Add(simpleOwnedPod("unit", "test"))
	validateNotSent(t, tc, sourcesv1alpha1.ApiServerSourceAddRefEventType)
}

func TestControllerAddEventWithGoodController(t *testing.T) {
	c, tc := makeController("apps/v1", "ReplicaSet")
	c.Add(simpleOwnedPod("unit", "test"))
	validateSent(t, tc, sourcesv1alpha1.ApiServerSourceAddRefEventType)
}

func TestControllerAddEventWithGoodControllerNoAPIVersion(t *testing.T) {
	c, tc := makeController("", "ReplicaSet")
	c.Add(simpleOwnedPod("unit", "test"))
	validateSent(t, tc, sourcesv1alpha1.ApiServerSourceAddRefEventType)
}

func TestControllerUpdateEventWithNoController(t *testing.T) {
	c, tc := makeController("v1", "Pod")
	c.Update(simplePod("unit", "test"))
	validateNotSent(t, tc, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestControllerUpdateEventWithWrongController(t *testing.T) {
	c, tc := makeController("v1", "Pod")
	c.Update(simpleOwnedPod("unit", "test"))
	validateNotSent(t, tc, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestControllerUpdateEventWithGoodController(t *testing.T) {
	c, tc := makeController("apps/v1", "ReplicaSet")
	c.Update(simpleOwnedPod("unit", "test"))
	validateSent(t, tc, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestControllerUpdateEventWithGoodControllerNoAPIVersion(t *testing.T) {
	c, tc := makeController("", "ReplicaSet")
	c.Update(simpleOwnedPod("unit", "test"))
	validateSent(t, tc, sourcesv1alpha1.ApiServerSourceUpdateRefEventType)
}

func TestControllerDeleteEventWithNoController(t *testing.T) {
	c, tc := makeController("v1", "Pod")
	c.Delete(simplePod("unit", "test"))
	validateNotSent(t, tc, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
}

func TestControllerDeleteEventWithWrongController(t *testing.T) {
	c, tc := makeController("v1", "Pod")
	c.Delete(simpleOwnedPod("unit", "test"))
	validateNotSent(t, tc, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
}

func TestControllerDeleteEventWithGoodController(t *testing.T) {
	c, tc := makeController("apps/v1", "ReplicaSet")
	c.Delete(simpleOwnedPod("unit", "test"))
	validateSent(t, tc, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
}

func TestControllerDeleteEventWithGoodControllerNoAPIVersion(t *testing.T) {
	c, tc := makeController("", "ReplicaSet")
	c.Delete(simpleOwnedPod("unit", "test"))
	validateSent(t, tc, sourcesv1alpha1.ApiServerSourceDeleteRefEventType)
}

func makeController(apiVersion, kind string) (*controllerFilter, *adaptertest.TestCloudEventsClient) {
	delegate, tc := makeRefAndTestingClient()
	return &controllerFilter{
		apiVersion: apiVersion,
		kind:       kind,
		delegate:   delegate,
	}, tc
}
