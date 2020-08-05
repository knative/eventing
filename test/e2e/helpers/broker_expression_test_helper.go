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
	"testing"

	"knative.dev/eventing/pkg/reconciler/sugar"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// TestBrokerWithExpressionTrigger If shouldLabelNamespace is set to true this test annotates the testing namespace so that a default broker is created.
// It then binds many triggers with different filtering patterns to the broker created by brokerCreator, and sends
// different events to the broker's address.
// Finally, it verifies that only the appropriate events are routed to the subscribers.
func TestBrokerWithExpressionTrigger(t *testing.T, brokerCreator BrokerCreator, shouldLabelNamespace bool) {
	client := testlib.Setup(t, true)
	defer testlib.TearDown(client)

	if shouldLabelNamespace {
		// Label namespace so that it creates the default broker.
		if err := client.LabelNamespace(map[string]string{sugar.InjectionLabelKey: sugar.InjectionEnabledLabelValue}); err != nil {
			t.Fatalf("Error annotating namespace: %v", err)
		}
	}

	brokerName := brokerCreator(client, "v1beta1")

	// Wait for broker ready.
	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)

	if shouldLabelNamespace {
		// Test if namespace reconciler would recreate broker once broker was deleted.
		if err := client.Eventing.EventingV1beta1().Brokers(client.Namespace).Delete(brokerName, &metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Can't delete default broker in namespace: %v", client.Namespace)
		}
		client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)
	}

	subscriberNamePass := "recordevents-expression-pass"
	eventTrackerPass, _ := recordevents.StartEventRecordOrFail(client, subscriberNamePass)
	defer eventTrackerPass.Cleanup()

	subscriberNameFail := "recordevents-expression-fail"
	eventTrackerFail, _ := recordevents.StartEventRecordOrFail(client, subscriberNameFail)
	defer eventTrackerFail.Cleanup()

	triggerNameOk := "trigger-expression-pass"
	client.CreateTriggerOrFailV1Beta1(triggerNameOk,
		resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberNamePass),
		resources.WithExpressionTriggerFilterV1Beta1("event.id != null"),
		resources.WithBrokerV1Beta1(brokerName),
	)

	triggerNameFail := "trigger-expression-fail"
	client.CreateTriggerOrFailV1Beta1(triggerNameFail,
		resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberNamePass),
		resources.WithExpressionTriggerFilterV1Beta1("event.id == null"),
		resources.WithBrokerV1Beta1(brokerName),
	)

	// Wait for all test resources to become ready before sending the events.
	client.WaitForAllTestResourcesReadyOrFail()

	client.SendEventToAddressable("sender", brokerName, testlib.BrokerTypeMeta, cetest.FullEvent())

	eventTrackerPass.AssertExact(1, recordevents.MatchEvent(cetest.HasSpecVersion(cloudevents.VersionV1)))
	eventTrackerFail.AssertNot(recordevents.MatchEvent(cetest.HasSpecVersion(cloudevents.VersionV1)))
}
