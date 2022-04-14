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
	"log"
	"os"
	"testing"

	"knative.dev/eventing/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/test/zipkin"
)

// RunMainTest initializes the flags to run the eventing upgrade tests, and runs the channel tests.
// This function needs to be exposed, so that test cases in other repositories can call the upgrade
// main tests in eventing.
func RunMainTest(m *testing.M) {
	test.InitializeEventingFlags()
	channelTestRunner = testlib.ComponentsTestRunner{
		ComponentFeatureMap: testlib.ChannelFeatureMap,
		ComponentsToTest:    test.EventingFlags.Channels,
	}
	os.Exit(func() int {
		// Any tests may SetupZipkinTracing, it will only actually be done once. This should be the ONLY
		// place that cleans it up. If an individual test calls this instead, then it will break other
		// tests that need the tracing in place.
		defer zipkin.CleanupZipkinTracingSetup(log.Printf)
		return m.Run()
	}())
}
