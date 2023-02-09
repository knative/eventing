/*
Copyright 2021 The Knative Authors

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

package svc

import (
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

// Deprecated: use knative.dev/reconciler-test/pkg/resources/service.GVR
var GVR = service.GVR

// Install will create a Service resource mapping :80 to :8080 on the provided
// selector for pods.
// Deprecated: use knative.dev/reconciler-test/pkg/resources/service.Install
func Install(name, selectorKey, selectorValue string) feature.StepFn {
	return service.Install(name, service.WithSelectors(map[string]string{
		selectorKey: selectorValue,
	}))
}

// AsKReference returns a KReference for a Service without namespace.
// Deprecated: use knative.dev/reconciler-test/pkg/resources/service.AsKReference
var AsKReference = service.AsKReference

// Deprecated: use knative.dev/reconciler-test/pkg/resources/service.AsTrackerReference
var AsTrackerReference = service.AsTrackerReference

// Deprecated: use knative.dev/reconciler-test/pkg/resources/service.AsDestinationRef
var AsDestinationRef = service.AsDestinationRef

// Deprecated: use knative.dev/reconciler-test/pkg/resources/service.Address
var Address = service.Address
