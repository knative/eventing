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
	rbacv1 "k8s.io/api/rbac/v1"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"fmt"
	"log"
	"os"
	"testing"

	"knative.dev/pkg/test/zipkin"

	"knative.dev/eventing/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
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
			ComponentsToTest: test.EventingFlags.Sources,
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

func addSourcesInitializers(){
	sourcesTestRunner.AddComponentSetupClientOption(
		testlib.ApiServerSourceTypeMeta, func(t *testing.T, client *testlib.Client) {
			const (
				baseApiServerSourceName = "conf-api-server-source"
				roleName              	= "event-watcher-r"
				serviceAccountName    	= "event-watcher-sa"
				sinkPodName				= "conf-source-sink"
			)

			// creates ServiceAccount and RoleBinding with a role for reading pods and events
			r := resources.Role(roleName,
				resources.WithRuleForRole(&rbacv1.PolicyRule{
					APIGroups: []string{""},
					Resources: []string{"events", "pods"},
					Verbs:     []string{"get", "list", "watch"}}))
			client.CreateServiceAccountOrFail(serviceAccountName)
			client.CreateRoleOrFail(r)
			client.CreateRoleBindingOrFail(
				serviceAccountName,
				testlib.RoleKind,
				roleName,
				fmt.Sprintf("%s-%s", serviceAccountName, roleName),
				client.Namespace,
			)

			spec := sourcesv1alpha2.ApiServerSourceSpec{
				Resources: []sourcesv1alpha2.APIVersionKindSelector{{
					APIVersion: "v1",
					Kind:       "Event",
				}},
				ServiceAccountName: serviceAccountName,
			}
			spec.Sink = duckv1.Destination{Ref: resources.ServiceKRef(sinkPodName)}

			apiServerSource := eventingtesting.NewApiServerSource(
				baseApiServerSourceName,
				client.Namespace,
				eventingtesting.WithApiServerSourceSpec(spec),
			)

			client.CreateApiServerSourceOrFail(apiServerSource)

			// wait for all test resources to be ready
			client.WaitForAllTestResourcesReadyOrFail()
	})


}
