/*
Copyright 2020 The Knative Authors

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

package upgrade

import (
	"context"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/upgrade/prober"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

func ContinualTest() pkgupgrade.BackgroundOperation {
	ctx := context.Background()
	var client *testlib.Client
	var probe prober.Prober
	return pkgupgrade.NewBackgroundVerification("EventingContinualTest",
		func(c pkgupgrade.Context) {
			// setup
			client = testlib.Setup(c.T, false)
			config := prober.NewConfig(client.Namespace)
			probe = prober.RunEventProber(ctx, c.Log, client, config)
		},
		func(c pkgupgrade.Context) {
			// verify
			defer testlib.TearDown(client)
			prober.AssertEventProber(ctx, c.T, probe)
		},
	)
}
