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

package pod

import (
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/pod"
)

// Install will create a Pod with defaults that can be overwritten by
// the With* methods.
// Deprecated, use knative.dev/reconciler-test/pkg/resources/pod.Install
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {

	return pod.Install(name, "gcr.io/knative-samples/helloworld-go",
		pod.WithLabels(map[string]string{"app": name}),
		pod.WithPort(8080))
}

// Deprecated, use knative.dev/reconciler-test/pkg/resources/pod.WithLabels
var WithLabels = pod.WithLabels

// Deprecated, use knative.dev/reconciler-test/pkg/resources/pod.WithNamespace
var WithNamespace = pod.WithNamespace
