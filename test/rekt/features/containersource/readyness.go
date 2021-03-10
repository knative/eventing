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
	"fmt"

	"knative.dev/eventing/test/rekt/features"
	"knative.dev/eventing/test/rekt/resources/containersource"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/reconciler-test/pkg/feature"
)

// GoesReady returns a feature testing if a containersource becomes ready.
func GoesReady(name string, cfg ...containersource.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("ContainerSource goes ready.")

	sink := feature.MakeRandomK8sName("sink")
	f.Setup("install a Service", svc.Install(sink, "app", "rekt"))

	f.Setup(fmt.Sprintf("install a ContainerSource named %q", name),
		containersource.Install(name, svc.AsDestinationRef(sink), cfg...))

	f.Assert("ContainerSource is ready", containersource.IsReady(name, features.Interval, features.Timeout))

	return f
}
