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
	"strings"
	"testing"

	v1 "knative.dev/pkg/apis/duck/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/reconciler"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/lib/recordevents"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

const brokerName = "br"

// Creates a Broker with the given name.
type BrokerCreator func(client *testlib.Client, name string)

// This tests if the broker control plane:
// 1. Trigger can be created before Broker (with attributes filter)
// 2. Broker can be created and progresses to Ready
// 3. Ready Broker is Addressable
// 4. Broker.Spec.Config is immutable
// 5. Trigger with Ready broker progresses to Ready
// 6. Trigger with no broker, updated with broker, updates status to include subscriberURI
// 7. Ready Trigger includes status.subscriberUri
func BrokerV1ControlPlaneTest(
	t *testing.T,
	brokerCreator BrokerCreator,
	setupClient ...testlib.SetupClientOption,
) {

	client := testlib.Setup(t, false, setupClient...)
	defer testlib.TearDown(client)
	triggerNoBroker := "trigger-no-broker"
	triggerWithBroker := "trigger-with-broker"

	t.Run("Trigger V1 can be created before Broker (with attributes filter)", func(t *testing.T) {
		triggerV1BeforeBrokerHelper(triggerNoBroker, client)
	})

	t.Run("Broker V1 can be created and progresses to Ready", func(t *testing.T) {
		brokerV1CreatedToReadyHelper(brokerName, client, brokerCreator)
	})

	t.Run("Ready Broker V1 is Addressable", func(t *testing.T) {
		readyBrokerV1AvailableHelper(t, brokerName, client)
	})

	t.Run("Ready Broker.Spec.Config is immutable", func(t *testing.T) {
		brokerV1ConfigCanNotBeUpdated(t, brokerName, client)
	})

	t.Run("Trigger V1 with Ready broker progresses to Ready", func(t *testing.T) {
		triggerV1ReadyBrokerReadyHelper(triggerWithBroker, brokerName, client)
	})

	t.Run("Ready Trigger V1 (no Broker) set Broker and includes status.subscriber Uri", func(t *testing.T) {
		triggerV1CanNotUpdateBroker(t, triggerNoBroker, brokerName+"different", client)
	})

	t.Run("Ready Trigger V1 includes status.subscriber Uri", func(t *testing.T) {
		triggerV1ReadyIncludesSubURI(t, triggerWithBroker, client)
	})
}

func triggerV1BeforeBrokerHelper(triggerName string, client *testlib.Client) {
	const etLogger = "logger"
	const loggerPodName = "logger-pod"

	_ = recordevents.DeployEventRecordOrFail(context.TODO(), client, loggerPodName)
	client.WaitForAllTestResourcesReadyOrFail(context.Background()) // Can't do this for the trigger because it's not 'ready' yet
	client.CreateTriggerOrFail(triggerName,
		resources.WithAttributesTriggerFilter(eventingv1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTrigger(loggerPodName),
		resources.WithBroker(brokerName),
	)
}

func brokerV1CreatedToReadyHelper(brokerName string, client *testlib.Client, brokerCreator BrokerCreator) {
	brokerCreator(client, brokerName)
	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)
}

func readyBrokerV1AvailableHelper(t *testing.T, brokerName string, client *testlib.Client) {
	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)
	obj := resources.NewMetaResource(brokerName, client.Namespace, testlib.BrokerTypeMeta)
	_, err := duck.GetAddressableURI(client.Dynamic, obj)
	if err != nil {
		t.Fatal("Broker is not addressable", err)
	}
}

func brokerV1ConfigCanNotBeUpdated(t *testing.T, brokerName string, client *testlib.Client) {
	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)
	err := client.RetryWebhookErrors(func(i int) error {
		broker, err := client.Eventing.EventingV1().Brokers(client.Namespace).Get(context.Background(), brokerName, metav1.GetOptions{})
		if err != nil {
			t.Logf("Error: Could not get broker %s: %v", brokerName, err)
			return err
		}
		broker.Spec = eventingv1.BrokerSpec{
			Config: &v1.KReference{
				Kind:       "kind",
				Namespace:  "namespace",
				Name:       "name",
				APIVersion: "apiversion",
			},
		}

		_, err = client.Eventing.EventingV1().Brokers(client.Namespace).Update(context.Background(), broker, metav1.UpdateOptions{})
		return err
	})
	if err == nil {
		t.Fatalf("Error: Was able to update the broker.Spec.Config %s", brokerName)
	}
}

func triggerV1ReadyBrokerReadyHelper(triggerName, brokerName string, client *testlib.Client) {
	const etLogger = "logger"
	const loggerPodName = "logger-pod"

	trigger := client.CreateTriggerOrFail(triggerName,
		resources.WithAttributesTriggerFilter(eventingv1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTrigger(loggerPodName),
		resources.WithBroker(brokerName),
	)

	client.WaitForResourceReadyOrFail(trigger.Name, testlib.TriggerTypeMeta)
}

func triggerV1CanNotUpdateBroker(t *testing.T, triggerName, brokerName string, client *testlib.Client) {
	err := client.RetryWebhookErrors(func(i int) error {
		return reconciler.RetryUpdateConflicts(func(attempts int) (err error) {
			tr, err := client.Eventing.EventingV1().Triggers(client.Namespace).Get(context.Background(), triggerName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Error: Could not get trigger %s: %v", triggerName, err)
			}
			tr.Spec.Broker = brokerName
			_, e := client.Eventing.EventingV1().Triggers(client.Namespace).Update(context.Background(), tr, metav1.UpdateOptions{})
			return e
		})
	})
	if err == nil {
		t.Fatalf("Error: Was able to update the trigger.Spec.Broker %s", triggerName)
	}

	if !strings.Contains(err.Error(), "Immutable fields changed (-old +new): broker, spec") {
		t.Fatalf("Unexpected failure to update trigger, expected Immutable fields changed  (-old +new): broker, spec But got: %v", err)
	}
}

func triggerV1ReadyIncludesSubURI(t *testing.T, triggerName string, client *testlib.Client) {
	client.WaitForResourceReadyOrFail(triggerName, testlib.TriggerTypeMeta)
	var tr *eventingv1.Trigger
	triggers := client.Eventing.EventingV1().Triggers(client.Namespace)
	err := client.RetryWebhookErrors(func(attempts int) (err error) {
		var e error
		client.T.Logf("Getting v1 trigger %s", triggerName)
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
