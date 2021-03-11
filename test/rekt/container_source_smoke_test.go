// +build e2e

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

package rekt

import (
	"fmt"
	"testing"

	"knative.dev/eventing/test/rekt/features/containersource"
	cs "knative.dev/eventing/test/rekt/resources/containersource"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/reconciler-test/pkg/feature"
)

// TestSmoke_ContainerSource
func TestSmoke_ContainerSource(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		f := containersource.GoesReady(name)

		sink := feature.MakeRandomK8sName("sink")
		f.Setup("install a Service", svc.Install(sink, "app", "rekt"))

		f.Setup(fmt.Sprintf("install a ContainerSource named %q", name),
			cs.Install(name, svc.AsDestinationRef(sink)))

		env.Test(ctx, t, f)
	}
}
