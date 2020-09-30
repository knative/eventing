// +build e2e

/*
Copyright 2019 The Knative Authors
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

package conformance

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"knative.dev/eventing/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/setupclientoptions"
	"knative.dev/pkg/test/zipkin"
)

const (
	roleName                = "event-watcher-r"
	serviceAccountName      = "event-watcher-sa"
	recordEventsAPIPodName  = "api-server-source-logger-pod"
	recordEventsPingPodName = "ping-source-logger-pod"
)

var channelTestRunner testlib.ComponentsTestRunner
var sourcesTestRunner testlib.ComponentsTestRunner
var brokerTestRunner testlib.ComponentsTestRunner
var brokerClass string

func TestMain(m *testing.M) {
	os.Exit(func() int {
		test.InitializeEventingFlags()
		channelTestRunner = testlib.ComponentsTestRunner{
			ComponentFeatureMap: testlib.ChannelFeatureMap,
			ComponentsToTest:    test.EventingFlags.Channels,
		}
		sourcesTestRunner = testlib.ComponentsTestRunner{
			ComponentFeatureMap: testlib.SourceFeatureMap,
			ComponentsToTest:    test.EventingFlags.Sources,
		}
		brokerTestRunner = testlib.ComponentsTestRunner{
			ComponentFeatureMap: testlib.BrokerFeatureMap,
			ComponentsToTest:    test.EventingFlags.Brokers,
			ComponentName:       test.EventingFlags.BrokerName,
			ComponentNamespace:  test.EventingFlags.BrokerNamespace,
		}
		brokerClass = test.EventingFlags.BrokerClass

		addSourcesInitializers()
		// Any tests may SetupZipkinTracing, it will only actually be done once. This should be the ONLY
		// place that cleans it up. If an individual test calls this instead, then it will break other
		// tests that need the tracing in place.
		defer zipkin.CleanupZipkinTracingSetup(log.Printf)
		defer testlib.ExportLogs(testlib.SystemLogsDir, resources.SystemNamespace)

		return m.Run()
	}())
}

func addSourcesInitializers() {

	ctx := context.Background()

	apiSrcName := strings.ToLower(testlib.ApiServerSourceTypeMeta.Kind)
	pingSrcName := strings.ToLower(testlib.PingSourceTypeMeta.Kind)
	sourcesTestRunner.AddComponentSetupClientOption(
		testlib.ApiServerSourceTypeMeta,
		setupclientoptions.ApiServerSourceV1ClientSetupOption(
			ctx, apiSrcName, "Reference",
			recordEventsAPIPodName, roleName, serviceAccountName),
	)
	sourcesTestRunner.AddComponentSetupClientOption(
		testlib.PingSourceTypeMeta,
		setupclientoptions.PingSourceV1B1ClientSetupOption(
			ctx, pingSrcName, recordEventsPingPodName),
	)
}
