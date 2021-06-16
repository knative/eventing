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

package e2e

import (
	"os"
	"testing"

	"knative.dev/eventing/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/system"
)

var setup = testlib.Setup
var tearDown = testlib.TearDown
var channelTestRunner testlib.ComponentsTestRunner
var brokerClass string

func TestMain(m *testing.M) {
	test.InitializeEventingFlags()
	testlib.ReuseNamespace = test.EventingFlags.ReuseNamespace
	channelTestRunner = testlib.ComponentsTestRunner{
		ComponentFeatureMap: testlib.ChannelFeatureMap,
		ComponentsToTest:    test.EventingFlags.Channels,
	}
	brokerClass = test.EventingFlags.BrokerClass

	exit := m.Run()

	// Collect logs only when test failed.
	if exit != 0 {
		testlib.ExportLogs(testlib.SystemLogsDir, system.Namespace())
	}

	os.Exit(exit)
}
