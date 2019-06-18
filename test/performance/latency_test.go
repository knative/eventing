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

	"github.com/knative/eventing/test/base/resources"
	"github.com/knative/eventing/test/common"
	"github.com/knative/test-infra/shared/junit"
	"github.com/knative/test-infra/shared/testgrid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLatencyForNatssBrokerTrigger(t *testing.T) {
	testLatencyForBrokerTrigger(t, common.NatssChannelTypeMeta)
}

func TestLatencyForKafkaBrokerTrigger(t *testing.T) {
	testLatencyForBrokerTrigger(t, common.KafkaChannelTypeMeta)
}

func TestLatencyForInMemoryBrokerTrigger(t *testing.T) {
	testLatencyForBrokerTrigger(t, common.InMemoryChannelTypeMeta)
}

func testLatencyForBrokerTrigger(t *testing.T, channelTypeMeta *metav1.TypeMeta) {
	const (
		senderName    = "perf-latency-sender"
		brokerName    = "perf-latency-broker"
		triggerName   = "perf-latency-trigger"
		saIngressName = "eventing-broker-ingress"
		saFilterName  = "eventing-broker-filter"

		// This ClusterRole is installed in Knative Eventing setup, see https://github.com/knative/eventing/tree/master/docs/broker#manual-setup.
		crIngressName = "eventing-broker-ingress"
		crFilterName  = "eventing-broker-filter"

		latencyPodName = "perf-latency-pod"
	)

	client := common.Setup(t, false)
	defer common.TearDown(client)

	// creates ServiceAccount and ClusterRoleBinding with default cluster-admin role
	client.CreateServiceAccountAndBindingOrFail(saIngressName, crIngressName)
	client.CreateServiceAccountAndBindingOrFail(saFilterName, crFilterName)

	// create a new broker
	client.CreateBrokerOrFail(brokerName, channelTypeMeta, "")
	client.WaitForResourceReady(brokerName, common.BrokerTypeMeta)
	brokerURL, err := client.GetAddressableURI(brokerName, common.BrokerTypeMeta)
	if err != nil {
		t.Fatalf("failed to get the URL for the broker: %v", err)
	}

	// create event latency measurement service
	latencyPod := resources.EventLatencyPod(latencyPodName, brokerURL, 1000)
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
