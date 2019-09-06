// +build performance

/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package performance

import (
	"log"
	"sort"
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	"knative.dev/test-infra/shared/junit"
	"knative.dev/test-infra/shared/testgrid"
)

func TestLatencyForInMemoryBrokerTrigger(t *testing.T) {
	testLatencyForBrokerTrigger(t, common.GetChannelTypeMeta(resources.InMemoryChannelKind))
}

func testLatencyForBrokerTrigger(t *testing.T, channelTypeMeta *metav1.TypeMeta) {
	const (
		senderName  = "perf-latency-sender"
		brokerName  = "perf-latency-broker"
		triggerName = "perf-latency-trigger"

		latencyPodName = "perf-latency-pod"

		// the number of events we want to send to the Broker in parallel.
		// TODO(Fredy-Z): for now this test will only be run against a single-node cluster, and as we want to calculate
		//                the sample percentile for the test results, 1000 seems to be a reasonable number. In the future
		//                we might want to change it to a higher number for more complicated tests.
		eventCount = 1000
	)

	client := common.Setup(t, false)
	defer common.TearDown(client)

	// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
	client.CreateRBACResourcesForBrokers()

	// create a new broker
	client.CreateBrokerOrFail(brokerName, channelTypeMeta)
	client.WaitForResourceReady(brokerName, common.BrokerTypeMeta)
	brokerURL, err := client.GetAddressableURI(brokerName, common.BrokerTypeMeta)
	if err != nil {
		t.Fatalf("failed to get the URL for the broker: %v", err)
	}

	// create event latency measurement service
	latencyPod := resources.EventLatencyPod(latencyPodName, brokerURL, eventCount)
	client.CreatePodOrFail(latencyPod, common.WithService(latencyPodName))

	// create the trigger to receive the event and forward it back to the event latency measurement service
	client.CreateTriggerOrFail(
		triggerName,
		resources.WithBroker(brokerName),
		resources.WithSubscriberRefForTrigger(latencyPodName),
	)

	// wait for all test resources to be ready, so that we can start sending events
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// parse test result from the Pod log
	res, err := ParseTestResultFromLog(client.Kube, latencyPodName, latencyPod.Spec.Containers[0].Name, client.Namespace)
	if err != nil {
		t.Fatalf("failed to get the test result: %v", err)
	}

	// fail the test directly if the result is TestFail
	if testResult, ok := res[TestResultKey]; ok {
		if testResult == TestFail {
			t.Fatalf("error happens when running test in the pod: %s", res[TestFailReason])
		}
	}

	// collect the metricNames and sort them
	metricNames := make([]string, 0, len(res))
	for key := range res {
		if key == TestResultKey {
			continue
		}
		metricNames = append(metricNames, key)
	}
	sort.Strings(metricNames)

	// create latency metrics and save them as XML files that can be parsed by Testgrid
	var tc []junit.TestCase
	for _, metricName := range metricNames {
		metricValue := res[metricName]
		floatMetricValue, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			t.Fatalf("unknown metric value %s for %s", metricValue, metricName)
		}
		tc = append(tc, CreatePerfTestCase(float32(floatMetricValue), metricName, t.Name()))
	}

	if err := testgrid.CreateXMLOutput(tc, t.Name()); err != nil {
		log.Fatalf("Cannot create output xml: %v", err)
	}
}
