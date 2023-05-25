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

package containersource

import (
	"knative.dev/eventing/test/rekt/resources/containersource"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

// GoesReady returns a feature testing if a containersource becomes ready.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("ContainerSource goes ready.")

	sink := feature.MakeRandomK8sName("sink")
	f.Setup("install a service", service.Install(sink,
		service.WithSelectors(map[string]string{"app": "rekt"})))

	cfg = append(cfg, containersource.WithSink(service.AsDestinationRef(sink)))

	f.Setup("install a ContainerSource", containersource.Install(name, cfg...))

	f.Requirement("ContainerSource is ready", containersource.IsReady(name))

	f.Stable("ContainerSource")

	return f
}
