/*
Copyright 2026 The Knative Authors

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

package backoff_max

import (
	"net/http"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/utils/pointer"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

// BrokerToTrigger verifies that BackoffMax caps Trigger delivery retries.
func BrokerToTrigger() *feature.Feature {
	f := feature.NewFeatureNamed("Delivery backoff maximum through Trigger")

	brokerName := feature.MakeRandomK8sName("backoff-max-broker")
	triggerName := feature.MakeRandomK8sName("backoff-max-trigger")
	receiverName := feature.MakeRandomK8sName("backoff-max-receiver")
	senderName := feature.MakeRandomK8sName("backoff-max-sender")
	event := cetest.FullEvent()
	backoffPolicy := eventingduckv1.BackoffPolicyExponential

	f.Setup("install receiver", eventshub.Install(
		receiverName,
		eventshub.StartReceiver,
		eventshub.DropFirstN(4),
		eventshub.DropEventsResponseCode(http.StatusServiceUnavailable),
	))
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Setup("install trigger", trigger.Install(
		triggerName,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(receiverName), ""),
		trigger.WithRetry(4, &backoffPolicy, pointer.String("PT1S")),
		trigger.WithBackoffMax("PT2S"),
	))
	f.Requirement("trigger is ready", trigger.IsReady(triggerName))
	f.Assert("send event", eventshub.Install(
		senderName,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	assertBackoffMax(f, receiverName, event.ID())

	return f
}
