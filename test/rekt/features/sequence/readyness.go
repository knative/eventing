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

package sequence

import (
	"strconv"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/eventing/test/rekt/resources/sequence"
)

// GoesReady returns a feature testing if a Sequence becomes ready with 3 steps.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("Sequence goes ready.")

	{
		reply := feature.MakeRandomK8sName("reply")
		f.Setup("install a reply service", service.Install(reply,
			service.WithSelectors(map[string]string{"app": "rekt"})))
		cfg = append(cfg, sequence.WithReply(service.AsKReference(reply), ""))
	}

	for i := 0; i < 3; i++ {
		// step
		step := feature.MakeRandomK8sName("step" + strconv.Itoa(i))
		f.Setup("install step "+strconv.Itoa(i), service.Install(step,
			service.WithSelectors(map[string]string{"app": "rekt"})))
		cfg = append(cfg, sequence.WithStep(service.AsKReference(step), ""))
	}

	f.Setup("install a Sequence", sequence.Install(name, cfg...))

	f.Requirement("Sequence is ready", sequence.IsReady(name))

	f.Stable("Sequence")

	return f
}
