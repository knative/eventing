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

	"knative.dev/eventing/test/rekt/resources/sequence"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

// GoesReady returns a feature testing if a Sequence becomes ready with 3 steps.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("Parallel goes ready.")

	{
		reply := feature.MakeRandomK8sName("reply")
		f.Setup("install a reply service", svc.Install(reply, "app", "rekt"))
		cfg = append(cfg, sequence.WithReply(svc.AsKReference(reply), ""))
	}

	for i := 0; i < 3; i++ {
		// step
		step := feature.MakeRandomK8sName("step" + strconv.Itoa(i))
		f.Setup("install step "+strconv.Itoa(i), svc.Install(step, "app", "rekt"))
		cfg = append(cfg, sequence.WithStep(svc.AsKReference(step), ""))
	}

	f.Setup("install a Sequence", sequence.Install(name, cfg...))

	f.Requirement("Sequence is ready", sequence.IsReady(name))

	f.Stable("Sequence")

	return f
}
