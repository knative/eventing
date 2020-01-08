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

package helpers

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgtest "knative.dev/pkg/test"
)

func SetupPerformanceImageRBAC(client *lib.Client) {
	client.CreateServiceAccountOrFail(resources.PerfServiceAccount)
	client.CreateClusterRoleOrFail(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "perf-eventing-list-pods",
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Verbs:     []string{"get", "watch", "list"},
			Resources: []string{"nodes", "pods"},
		}},
	})
	client.CreateClusterRoleBindingOrFail(resources.PerfServiceAccount, "perf-eventing-list-pods", "perf-eventing-list-pods")
}

var sentEventsRegex = regexp.MustCompile("Sent count: ([0-9.,]+)")
var acceptedEventsRegex = regexp.MustCompile("Accepted count: ([0-9.,]+)")
var receivedEventsRegex = regexp.MustCompile("Received count: ([0-9.,]+)")
var publishFailuresEventsRegex = regexp.MustCompile("Publish failure count: ([0-9.,]+)")
var deliveryFailuresEventsRegex = regexp.MustCompile("Delivery failure count: ([0-9.,]+)")

type PerformanceImageResults struct {
	SentEvents             uint64
	AcceptedEvents         uint64
	ReceivedEvents         uint64
	PublishFailuresEvents  uint64
	DeliveryFailuresEvents uint64
}

func getLogCount(log string, regex *regexp.Regexp) (uint64, error) {
	countUnparsed := regex.FindAllStringSubmatch(log, 1)
	if len(countUnparsed) == 0 {
		return 0, fmt.Errorf("no match for regex %s in log %s", regex.String(), log)
	}

	count, err := strconv.ParseUint(countUnparsed[0][1], 10, 64)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func parseLog(log string) (*PerformanceImageResults, error) {
	sentEvents, err := getLogCount(log, sentEventsRegex)
	if err != nil {
		return nil, err
	}
	acceptedEvents, err := getLogCount(log, acceptedEventsRegex)
	if err != nil {
		return nil, err
	}
	receivedEvents, err := getLogCount(log, receivedEventsRegex)
	if err != nil {
		return nil, err
	}
	publishFailuresEvents, err := getLogCount(log, publishFailuresEventsRegex)
	if err != nil {
		return nil, err
	}
	deliveryFailuresEvents, err := getLogCount(log, deliveryFailuresEventsRegex)
	if err != nil {
		return nil, err
	}

	return &PerformanceImageResults{
		sentEvents,
		acceptedEvents,
		receivedEvents,
		publishFailuresEvents,
		deliveryFailuresEvents,
	}, nil
}

// This function setups RBAC, services and aggregator pod. It DOESN'T setup channel, broker nor sender and receiver pod(s). The setup of these resources MUST be done inside `setupEnv` function
func TestWithPerformanceImage(st *testing.T, expectedAggregatorRecords int, setupEnv func(t *testing.T, consumerHostname string, aggregatorHostname string, client *lib.Client), assertResults func(*testing.T, *PerformanceImageResults)) {
	client := lib.Setup(st, true)
	defer lib.TearDown(client)

	// RBAC
	SetupPerformanceImageRBAC(client)

	// Services
	aggregatorService := client.CreateServiceOrFail(resources.PerformanceAggregatorService())
	consumerService := client.CreateServiceOrFail(resources.PerformanceConsumerService())

	// Aggregator
	client.CreatePodOrFail(resources.PerformanceImageAggregatorPod(expectedAggregatorRecords, false))

	// Call user function to setup the test
	setupEnv(st, duck.GetServiceHostname(consumerService), duck.GetServiceHostname(aggregatorService), client)

	// Wait for everything ready in test namespace
	client.WaitForAllTestResourcesReadyOrFail()

	// Wait for aggregator pod to finish
	err := pkgtest.WaitForPodState(client.Kube, func(pod *corev1.Pod) (b bool, e error) {
		if pod.Status.Phase == corev1.PodFailed {
			return true, fmt.Errorf("aggregator pod failed with message %s", pod.Status.Message)
		} else if pod.Status.Phase != corev1.PodSucceeded {
			return false, nil
		}
		return true, nil
	}, "perf-aggregator", client.Namespace)
	if err != nil {
		st.Fatal(err)
	}

	aggregatorLog, err := client.GetLog("perf-aggregator")
	if err != nil {
		st.Fatal(err)
	}

	results, err := parseLog(aggregatorLog)
	if err != nil {
		st.Fatal(err)
	}

	assertResults(st, results)
}
