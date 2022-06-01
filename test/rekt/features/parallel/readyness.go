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

package parallel

import (
	"strconv"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"

	"knative.dev/eventing/test/rekt/resources/parallel"
)

// GoesReady returns a feature testing if a Parallel becomes ready with 3 branches.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("Parallel goes ready.")

	{
		reply := feature.MakeRandomK8sName("reply")
		f.Setup("install a reply service", svc.Install(reply, "app", "rekt"))
		cfg = append(cfg, parallel.WithReply(svc.AsKReference(reply), ""))
	}

	for i := 0; i < 3; i++ {
		// Filter
		filter := feature.MakeRandomK8sName("subscriber" + strconv.Itoa(i))
		f.Setup("install filter "+strconv.Itoa(i), svc.Install(filter, "app", "rekt"))
		cfg = append(cfg, parallel.WithFilterAt(i, svc.AsKReference(filter), ""))

		// Subscriber
		subscriber := feature.MakeRandomK8sName("subscriber" + strconv.Itoa(i))
		f.Setup("install subscriber "+strconv.Itoa(i), svc.Install(subscriber, "app", "rekt"))
		cfg = append(cfg, parallel.WithSubscriberAt(i, svc.AsKReference(subscriber), ""))

		// Reply
		reply := feature.MakeRandomK8sName("reply" + strconv.Itoa(i))
		f.Setup("install reply "+strconv.Itoa(i), svc.Install(reply, "app", "rekt"))
		cfg = append(cfg, parallel.WithReplyAt(i, svc.AsKReference(reply), ""))
	}

	f.Setup("install a Parallel", parallel.Install(name, cfg...))

	f.Requirement("Parallel is ready", parallel.IsReady(name))

	f.Stable("Parallel")

	return f
}

// GoesReadyWithoutFilters returns a feature testing if a Parallel becomes ready without filters
func GoesReadyWithoutFilters(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("Parallel goes ready.")

	// Subscriber
	subscriber := feature.MakeRandomK8sName("subscriber")
	f.Setup("install subscriber", svc.Install(subscriber, "app", "rekt"))
	cfg = append(cfg, parallel.WithSubscriberAt(0, svc.AsKReference(subscriber), ""))

	f.Setup("install a Parallel", parallel.Install(name, cfg...))

	f.Requirement("Parallel is ready", parallel.IsReady(name))

	f.Stable("Parallel")

	return f
}
