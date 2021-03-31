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

package sinkbinding

import (
	"knative.dev/eventing/test/rekt/resources/sinkbinding"
	"knative.dev/reconciler-test/pkg/feature"
)

// GoesReady returns a feature testing if a SinkBinding becomes ready.
func GoesReady(name string) *feature.Feature {
	f := feature.NewFeatureNamed("SinkBinding goes ready.")

	f.Requirement("SinkBinding is ready", sinkbinding.IsReady(name))

	f.Stable("sinkbinding")

	return f
}
