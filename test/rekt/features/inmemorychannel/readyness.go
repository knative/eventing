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

package inmemorychannel

import (
	"fmt"

	"knative.dev/eventing/test/rekt/resources/inmemorychannel"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

// GoesReady returns a feature testing if a InMemoryChannel becomes ready.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("InMemoryChannel goes ready.")

	f.Setup(fmt.Sprintf("install a InMemoryChannel named %q", name), inmemorychannel.Install(name, cfg...))

	f.Requirement("InMemoryChannel is ready", inmemorychannel.IsReady(name))

	f.Stable("InMemoryChannel").
		Must("be addressable", inmemorychannel.IsAddressable(name))

	return f
}
