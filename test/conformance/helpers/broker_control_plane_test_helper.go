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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

func BrokerV1Beta1ControlPlaneTestHelperWithChannelTestRunner(
	t *testing.T,
	brokerClass string,
	channelTestRunner lib.ChannelTestRunner,
	setupClient ...lib.SetupClientOption,
) {
	channelTestRunner.RunTests(t, lib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(t, true, setupClient...)
		defer lib.TearDown(client)

		t.Run("Trigger V1Beta1 can be crated before Broker (with attributes filter)", func(t *testing.T) {
			TriggerV1Beta1BeforeBrokerHelper(t, brokerClass, client, channel)
		})

		t.Run("Broker V1Beta1 can be created and progresses to Ready", func(t *testing.T) {
			BrokerV1Beta1CreatedToReadyHelper(t, brokerClass, client, channel)
		})

		t.Run("Ready Broker V1Beta1 is Addressable", func(t *testing.T) {
			ReadyBrokerV1Beta1AvailableHelper(t, brokerClass, client, channel)
		})

		t.Run("Trigger V1Beta1 with Ready broker progresses to Ready", func(t *testing.T) {
			TriggerV1Beta1ReadyBrokerReadyHelper(t, brokerClass, client, channel)
		})

		t.Run("Ready Trigger V1Beta1 includes status.subscriber Uri", func(t *testing.T) {
			TriggerV1Beta1ReadyIncludesSubURI(t, brokerClass, client, channel)
		})
	})

}

func TriggerV1Beta1BeforeBrokerHelper(t *testing.T, brokerClass string, client *lib.Client, channel metav1.TypeMeta) {
	const etLogger = "logger"
	const loggerPodName = "logger-pod"

	logPod := resources.EventRecordPod(loggerPodName)
	client.CreatePodOrFail(logPod, lib.WithService(loggerPodName))
	client.WaitForAllTestResourcesReadyOrFail() //Can't do this for the trigger because it's not 'ready' yet
	client.CreateTriggerOrFail("trigger-no-broker",
		resources.WithAttributesTriggerFilter(eventingv1beta1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTrigger(loggerPodName),
	)
	client.Tracker.Clean(true)
}

func BrokerV1Beta1CreatedToReadyHelper(t *testing.T, brokerClass string, client *lib.Client, channel metav1.TypeMeta) {
	const defaultCMPName = "eventing"

	// Create the Broker.
	if brokerClass == eventing.ChannelBrokerClassValue {
		// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
		client.CreateConfigMapPropagationOrFail(defaultCMPName)
		client.CreateRBACResourcesForBrokers()
	}

	// Create a configmap used by the broker.
	client.CreateBrokerConfigMapOrFail("br", &channel)

	broker := client.CreateBrokerV1Beta1OrFail(
		"br",
		resources.WithBrokerClassForBrokerV1Beta1(brokerClass),
		resources.WithConfigMapForBrokerConfig(),
	)

	client.WaitForResourceReadyOrFail(broker.Name, lib.BrokerTypeMeta)

}

func ReadyBrokerV1Beta1AvailableHelper(t *testing.T, brokerClass string, client *lib.Client, channel metav1.TypeMeta) {
	client.WaitForResourceReadyOrFail("br", lib.BrokerTypeMeta)
	obj := resources.NewMetaResource("br", client.Namespace, lib.BrokerTypeMeta)
	_, err := duck.GetAddressableURI(client.Dynamic, obj)
	if err != nil {
		t.Fatalf("Broker is not addressable %w", err)
	}
}

func TriggerV1Beta1ReadyBrokerReadyHelper(t *testing.T, brokerClass string, client *lib.Client, channel metav1.TypeMeta) {
	const etLogger = "logger"
	const loggerPodName = "logger-pod"
	const defaultCMPName = "eventing"

	trigger := client.CreateTriggerOrFail("trigger-with-broker",
		resources.WithAttributesTriggerFilter(eventingv1beta1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTrigger(loggerPodName),
		resources.WithBroker("br"),
	)

	client.WaitForResourceReadyOrFail(trigger.Name, lib.TriggerTypeMeta)
}

func TriggerV1Beta1ReadyIncludesSubURI(t *testing.T, brokerClass string, client *lib.Client, channel metav1.TypeMeta) {
	client.WaitForResourceReadyOrFail("trigger-with-broker", lib.TriggerTypeMeta)
	tr, err := client.Eventing.EventingV1beta1().Triggers(client.Namespace).Get("trigger-with-broker", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error: Could not get trigger %s: %w", "trigger-with-broker", err)
	}
	if tr.Status.SubscriberURI == nil {
		t.Fatalf("Error: trigger.Status.SubscriberURI is nil but resource reported Ready")
	}
}
