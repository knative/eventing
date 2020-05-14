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

package helpers

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

const (
	brokerName = "notdefaultbroker"
)

// If shouldLabelNamespace is set to true this test annotates the testing namespace so that a default broker is created.
// It then binds many triggers with different filtering patterns to the broker created by brokerCreator, and sends
// different events to the broker's address.
// Finally, it verifies that only the appropriate events are routed to the subscribers.
func TestTriggerNoBroker(t *testing.T, channel string, brokerCreator BrokerCreator) {
	client := lib.Setup(t, true)
	defer lib.TearDown(client)
	brokerName := strings.ToLower(channel)
	subscriberName := name("dumper", "", "", map[string]interface{}{})
	pod := resources.EventLoggerPod(subscriberName)
	client.CreatePodOrFail(pod, lib.WithService(subscriberName))
	client.CreateTriggerOrFailV1Beta1("testtrigger",
		resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberName),
		resources.WithBrokerV1Beta1(brokerName),
	)

	// Then make sure the trigger is marked as not ready since there's no broker.
	err := wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
		trigger, err := client.Eventing.EventingV1beta1().Triggers(client.Namespace).Get("testtrigger", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if ready := trigger.Status.GetTopLevelCondition(); ready != nil {
			if ready.Status == corev1.ConditionFalse && ready.Reason == "BrokerDoesNotExist" {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("Trigger status did not get marked as BrokerDoesNotExist: %s", err)
	}

	// Then create the Broker and just make sure they both come ready.
	if bn := brokerCreator(client); bn != brokerName {
		t.Fatalf("Broker created with unexpected name, wanted %q got %q", brokerName, bn)
	}

	// Wait for all test resources to become ready before sending the events.
	client.WaitForAllTestResourcesReadyOrFail()
}
