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
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/reconciler"

	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

// Creates a Broker with the given name.
type BrokerCreator func(client *testlib.Client, name string)

// This tests if the broker control plane:
// 1. Trigger can be created before Broker (with attributes filter)
// 2. Broker can be created and progresses to Ready
// 3. Ready Broker is Addressable
// 4. Trigger with Ready broker progresses to Ready
// 5. Trigger with no broker, updated with broker, updates status to include subscriberURI
// 6. Ready Trigger includes status.subscriberUri
func BrokerV1Beta1ControlPlaneTest(
	t *testing.T,
	brokerCreator BrokerCreator,
	setupClient ...testlib.SetupClientOption,
) {

	client := testlib.Setup(t, false, setupClient...)
	defer testlib.TearDown(client)
	brokerName := "br"
	triggerNoBroker := "trigger-no-broker"
	triggerWithBroker := "trigger-with-broker"

	t.Run("Trigger V1Beta1 can be crated before Broker (with attributes filter)", func(t *testing.T) {
		triggerV1Beta1BeforeBrokerHelper(triggerNoBroker, client)
	})

	t.Run("Broker V1Beta1 can be created and progresses to Ready", func(t *testing.T) {
		brokerV1Beta1CreatedToReadyHelper(brokerName, client, brokerCreator)
	})

	t.Run("Ready Broker V1Beta1 is Addressable", func(t *testing.T) {
		readyBrokerV1Beta1AvailableHelper(t, brokerName, client)
	})

	t.Run("Trigger V1Beta1 with Ready broker progresses to Ready", func(t *testing.T) {
		triggerV1Beta1ReadyBrokerReadyHelper(triggerWithBroker, brokerName, client)
	})

	t.Run("Ready Trigger V1Beta1 (no Broker) set Broker and includes status.subscriber Uri", func(t *testing.T) {
		triggerV1Beta1ReadyAfterBrokerIncludesSubURI(t, triggerNoBroker, brokerName, client)
	})

	t.Run("Ready Trigger V1Beta1 includes status.subscriber Uri", func(t *testing.T) {
		triggerV1Beta1ReadyIncludesSubURI(t, triggerWithBroker, client)
	})
}

func triggerV1Beta1BeforeBrokerHelper(triggerName string, client *testlib.Client) {
	const etLogger = "logger"
	const loggerPodName = "logger-pod"

	logPod := resources.EventRecordPod(loggerPodName)
	client.CreatePodOrFail(logPod, testlib.WithService(loggerPodName))
	client.WaitForAllTestResourcesReadyOrFail(context.Background()) // Can't do this for the trigger because it's not 'ready' yet
	client.CreateTriggerOrFailV1Beta1(triggerName,
		resources.WithAttributesTriggerFilterV1Beta1(eventingv1beta1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerPodName),
	)
}

func brokerV1Beta1CreatedToReadyHelper(brokerName string, client *testlib.Client, brokerCreator BrokerCreator) {

	brokerCreator(client, brokerName)

	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)
}

func readyBrokerV1Beta1AvailableHelper(t *testing.T, brokerName string, client *testlib.Client) {
	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)
	obj := resources.NewMetaResource(brokerName, client.Namespace, testlib.BrokerTypeMeta)
	_, err := duck.GetAddressableURI(client.Dynamic, obj)
	if err != nil {
		t.Fatalf("Broker is not addressable %v", err)
	}
}

func triggerV1Beta1ReadyBrokerReadyHelper(triggerName, brokerName string, client *testlib.Client) {
	const etLogger = "logger"
	const loggerPodName = "logger-pod"

	trigger := client.CreateTriggerOrFailV1Beta1(triggerName,
		resources.WithAttributesTriggerFilterV1Beta1(eventingv1beta1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerPodName),
		resources.WithBrokerV1Beta1(brokerName),
	)

	client.WaitForResourceReadyOrFail(trigger.Name, testlib.TriggerTypeMeta)
}

func triggerV1Beta1ReadyAfterBrokerIncludesSubURI(t *testing.T, triggerName, brokerName string, client *testlib.Client) {
	err := reconciler.RetryUpdateConflicts(func(attempts int) (err error) {
		tr, err := client.Eventing.EventingV1beta1().Triggers(client.Namespace).Get(context.Background(), triggerName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Error: Could not get trigger %s: %v", triggerName, err)
		}
		tr.Spec.Broker = brokerName
		_, e := client.Eventing.EventingV1beta1().Triggers(client.Namespace).Update(context.Background(), tr, metav1.UpdateOptions{})
		return e
	})
	if err != nil {
		t.Fatalf("Error: Could not update trigger %s: %v", triggerName, err)
	}

	client.WaitForResourceReadyOrFail(triggerName, testlib.TriggerTypeMeta)

	var tr *eventingv1beta1.Trigger
	triggers := client.Eventing.EventingV1beta1().Triggers(client.Namespace)
	err = client.RetryWebhookErrors(func(attempts int) (err error) {
		var e error
		client.T.Logf("Getting v1beta1 trigger %s", triggerName)
		tr, e = triggers.Get(context.Background(), triggerName, metav1.GetOptions{})
		if e != nil {
			client.T.Logf("Failed to get trigger %q: %v", triggerName, e)
		}
		return err
	})
	if err != nil {
		t.Fatalf("Error: Could not get trigger %s: %v", triggerName, err)
	}
	if tr.Status.SubscriberURI == nil {
		t.Fatalf("Error: trigger.Status.SubscriberURI is nil but Broker Addressable & Ready")
	}
}

func triggerV1Beta1ReadyIncludesSubURI(t *testing.T, triggerName string, client *testlib.Client) {
	client.WaitForResourceReadyOrFail(triggerName, testlib.TriggerTypeMeta)
	var tr *eventingv1beta1.Trigger
	triggers := client.Eventing.EventingV1beta1().Triggers(client.Namespace)
	err := client.RetryWebhookErrors(func(attempts int) (err error) {
		var e error
		client.T.Logf("Getting v1beta1 trigger %s", triggerName)
		tr, e = triggers.Get(context.Background(), triggerName, metav1.GetOptions{})
		if e != nil {
			client.T.Logf("Failed to get trigger %q: %v", triggerName, e)
		}
		return err
	})
	if err != nil {
		t.Fatalf("Error: Could not get trigger %s: %v", triggerName, err)
	}
	if tr.Status.SubscriberURI == nil {
		t.Fatalf("Error: trigger.Status.SubscriberURI is nil but resource reported Ready")
	}
}
