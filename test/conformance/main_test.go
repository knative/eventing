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
	"fmt"
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
	roleName            = "event-watcher-r"
	serviceAccountName  = "event-watcher-sa"
	recordEventsPodName = "api-server-source-logger-pod"
)

var channelTestRunner testlib.ComponentsTestRunner
var sourcesTestRunner testlib.ComponentsTestRunner
var brokerClass string
var brokerName string
var brokerNamespace string

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
		brokerClass = test.EventingFlags.BrokerClass
		brokerName = test.EventingFlags.BrokerName
		brokerNamespace = test.EventingFlags.BrokerNamespace

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
	apiSrcName := strings.ToLower(fmt.Sprintf("%s",
		testlib.ApiServerSourceTypeMeta.Kind))
	pingSrcName := strings.ToLower(fmt.Sprintf("%s",
		testlib.PingSourceTypeMeta.Kind))
	sourcesTestRunner.AddComponentSetupClientOption(
		testlib.ApiServerSourceTypeMeta,
		setupclientoptions.ApiServerSourceClientSetupOption(apiSrcName,
			"Reference",
			recordEventsPodName, roleName, serviceAccountName),
	)
	sourcesTestRunner.AddComponentSetupClientOption(
		testlib.PingSourceTypeMeta,
		setupclientoptions.PingSourceClientSetupOption(pingSrcName,
			recordEventsPodName),
	)
}
