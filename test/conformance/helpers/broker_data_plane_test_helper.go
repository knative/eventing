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
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/sender"

	ce "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
)

var podMeta = metav1.TypeMeta{
	Kind:       "Pod",
	APIVersion: "v1",
}

func BrokerDataPlaneSetupHelper(ctx context.Context, client *testlib.Client, brokerClass string, brokerTestRunner testlib.ComponentsTestRunner) *eventingv1beta1.Broker {
	var broker *eventingv1beta1.Broker
	brokerName := brokerTestRunner.ComponentName
	brokerNamespace := brokerTestRunner.ComponentNamespace
	if brokerName == "" || brokerNamespace == "" {
		brokerName = "br"
		config := client.CreateBrokerConfigMapOrFail(brokerName, &testlib.DefaultChannel)

		broker = client.CreateBrokerV1Beta1OrFail(
			brokerName,
			resources.WithBrokerClassForBrokerV1Beta1(brokerClass),
			resources.WithConfigForBrokerV1Beta1(config),
		)
		client.WaitForResourceReadyOrFail(broker.Name, testlib.BrokerTypeMeta)
	} else {
		brokers := client.Eventing.EventingV1beta1().Brokers(brokerNamespace)
		err := client.RetryWebhookErrors(func(attempts int) (err error) {
			var e error
			client.T.Logf("Getting v1beta1 broker %s", brokerName)
			broker, e = brokers.Get(context.Background(), brokerName, metav1.GetOptions{})
			if e != nil {
				client.T.Logf("Failed to get broker %q: %v", brokerName, e)
			}
			return err
		})
		if err != nil {
			client.T.Fatalf("Could not Get broker %s/%s: %v", brokerNamespace, brokerName, err)
		}
	}
	return broker
}

func BrokerDataPlaneNamespaceSetupOption(ctx context.Context, namespace string) testlib.SetupClientOption {
	return func(client *testlib.Client) {
		if namespace != "" {
			client.Kube.CoreV1().Namespaces().Delete(ctx, client.Namespace, metav1.DeleteOptions{})
			client.Namespace = namespace
		}
	}
}

//At ingress
//Supports CE 0.3 or CE 1.0 via HTTP
//Supports structured or Binary mode
//Respond with 2xx on good CE
//Respond with 400 on bad CE
//Reject non-POST requests to publish URI
func BrokerV1Beta1IngressDataPlaneTestHelper(
	ctx context.Context,
	t *testing.T,
	brokerClass string,
	brokerTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	brokerTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, source metav1.TypeMeta) {
		client := testlib.Setup(t, false, options...)
		defer testlib.TearDown(client)

		broker := BrokerDataPlaneSetupHelper(ctx, client, brokerClass, brokerTestRunner)
		triggerName := "trigger"
		loggerName := "logger-pod"
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, loggerName)
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		trigger := client.CreateTriggerOrFailV1Beta1(
			triggerName,
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1(eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter, nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerName),
		)

		client.WaitForResourceReadyOrFail(trigger.Name, testlib.TriggerTypeMeta)
		st.Run("Ingress Supports CE0.3", func(t *testing.T) {
			eventID := "CE0.3"
			event := ce.NewEvent()
			event.SetID(eventID)
			event.SetType(testlib.DefaultEventType)
			event.SetSource("0.3.event.sender.test.knative.dev")
			body := fmt.Sprintf(`{"msg":%q}`, eventID)

			if err := event.SetData(ce.ApplicationJSON, []byte(body)); err != nil {
				t.Fatal("Cannot set the payload of the event:", err.Error())
			}
			event.Context.AsV03()
			event.SetSpecVersion("0.3")
			senderName := "v03-test-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			originalEventMatcher := recordevents.MatchEvent(cetest.AllOf(
				cetest.HasId(eventID),
				cetest.HasSpecVersion("0.3"),
			))
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertAtLeast(1, originalEventMatcher)
		})

		st.Run("Ingress Supports CE1.0", func(t *testing.T) {
			eventID := "CE1.0"
			event := ce.NewEvent()
			event.SetID(eventID)
			event.SetType(testlib.DefaultEventType)
			event.SetSource("1.0.event.sender.test.knative.dev")
			body := fmt.Sprintf(`{"msg":%q}`, eventID)
			if err := event.SetData(ce.ApplicationJSON, []byte(body)); err != nil {
				t.Fatal("Cannot set the payload of the event:", err.Error())
			}

			senderName := "v10-test-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			originalEventMatcher := recordevents.MatchEvent(cetest.AllOf(
				cetest.HasId(eventID),
				cetest.HasSpecVersion("1.0"),
			))
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertAtLeast(1, originalEventMatcher)

		})

		st.Run("Ingress Supports Structured Mode", func(t *testing.T) {
			eventID := "Structured-Mode"
			event := ce.NewEvent()
			event.SetID(eventID)
			event.SetType(testlib.DefaultEventType)
			event.SetSource("structured.mode.event.sender.test.knative.dev")
			body := fmt.Sprintf(`{"msg":%q}`, eventID)
			if err := event.SetData(ce.ApplicationJSON, []byte(body)); err != nil {
				t.Fatal("Cannot set the payload of the event:", err.Error())
			}
			senderName := "structured-test-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			originalEventMatcher := recordevents.MatchEvent(cetest.AllOf(
				cetest.HasId(eventID),
			))
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertAtLeast(1, originalEventMatcher)
		})

		st.Run("Ingress Supports Binary Mode", func(t *testing.T) {
			eventID := "Binary-Mode"
			event := ce.NewEvent()
			event.SetID(eventID)
			event.SetType(testlib.DefaultEventType)
			event.SetSource("binary.mode.event.sender.test.knative.dev")
			body := fmt.Sprintf(`{"msg":%q}`, eventID)
			if err := event.SetData(ce.ApplicationJSON, []byte(body)); err != nil {
				t.Fatal("Cannot set the payload of the event:", err.Error())
			}
			senderName := "binary-test-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingBinary))
			originalEventMatcher := recordevents.MatchEvent(cetest.AllOf(
				cetest.HasId(eventID),
			))
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertAtLeast(1, originalEventMatcher)
		})

		st.Run("Respond with 2XX on good CE", func(t *testing.T) {
			eventID := "2hundred-on-good-ce"
			body := fmt.Sprintf(`{"msg":%q}`, eventID)
			responseSink := "http://" + client.GetServiceHost(loggerName)
			senderName := "twohundred-test-sender"
			client.SendRequestToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta,
				map[string]string{
					"ce-specversion": "1.0",
					"ce-type":        testlib.DefaultEventType,
					"ce-source":      "2xx.request.sender.test.knative.dev",
					"ce-id":          eventID,
					"content-type":   ce.ApplicationJSON,
				},
				body,
				sender.WithResponseSink(responseSink),
			)
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertExact(1, recordevents.MatchEvent(sender.MatchStatusCode(202))) // should probably be a range

		})
		//Respond with 400 on bad CE
		st.Run("Respond with 400 on bad CE", func(t *testing.T) {
			eventID := "four-hundred-on-bad-ce"
			body := ";la}{kjsdf;oai2095{}{}8234092349807asdfashdf"
			responseSink := "http://" + client.GetServiceHost(loggerName)
			senderName := "fourhundres-test-sender"
			client.SendRequestToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta,
				map[string]string{
					"ce-specversion": "9000.1", //its over 9,000!
					"ce-type":        testlib.DefaultEventType,
					"ce-source":      "400.request.sender.test.knative.dev",
					"ce-id":          eventID,
					"content-type":   ce.ApplicationJSON,
				},
				body,
				sender.WithResponseSink(responseSink))
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertExact(1, recordevents.MatchEvent(sender.MatchStatusCode(400)))
		})
	})
}

//At consumer
//No upgrade of version
//Attributes received should be the same as produced (attributes may be added)
//Events are filtered
//Events are delivered to multiple subscribers
//Deliveries succeed at least once
//Replies are accepted and delivered
//Replies that are unsuccessfully forwarded cause initial message to be redelivered (Very difficult to test, can be ignored)
func BrokerV1Beta1ConsumerDataPlaneTestHelper(
	ctx context.Context,
	t *testing.T,
	brokerClass string,
	brokerTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	brokerTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, source metav1.TypeMeta) {
		client := testlib.Setup(t, false, options...)
		defer testlib.TearDown(client)

		broker := BrokerDataPlaneSetupHelper(ctx, client, brokerClass, brokerTestRunner)

		triggerName := "trigger"
		secondTriggerName := "second-trigger"
		loggerName := "logger-pod"
		secondLoggerName := "second-logger-pod"
		transformerName := "transformer-pod"
		replySource := "origin-for-reply"
		eventID := "consumer-broker-tests"
		baseSource := "consumer-test-sender"
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, loggerName)
		secondTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, secondLoggerName)

		baseEvent := ce.NewEvent()
		baseEvent.SetID(eventID)
		baseEvent.SetType(testlib.DefaultEventType)
		baseEvent.SetSource(baseSource)
		baseEvent.SetSpecVersion("1.0")
		body := fmt.Sprintf(`{"msg":%q}`, eventID)
		if err := baseEvent.SetData(ce.ApplicationJSON, []byte(body)); err != nil {
			t.Fatal("Cannot set the payload of the baseEvent:", err.Error())
		}

		transformMsg := []byte(`{"msg":"Transformed!"}`)
		recordevents.DeployEventRecordOrFail(
			ctx,
			client,
			transformerName,
			recordevents.ReplyWithTransformedEvent(
				"reply-check-type",
				"reply-check-source",
				string(transformMsg),
			),
		)

		trigger := client.CreateTriggerOrFailV1Beta1(
			triggerName,
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1(eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter, nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerName),
		)

		client.WaitForResourceReadyOrFail(trigger.Name, testlib.TriggerTypeMeta)
		secondTrigger := client.CreateTriggerOrFailV1Beta1(
			secondTriggerName,
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1("filtered-event", eventingv1beta1.TriggerAnyFilter, nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(secondLoggerName),
		)
		client.WaitForResourceReadyOrFail(secondTrigger.Name, testlib.TriggerTypeMeta)

		transformTrigger := client.CreateTriggerOrFailV1Beta1(
			"transform-trigger",
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1(replySource, baseEvent.Type(), nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(transformerName),
		)
		client.WaitForResourceReadyOrFail(transformTrigger.Name, testlib.TriggerTypeMeta)
		replyTrigger := client.CreateTriggerOrFailV1Beta1(
			"reply-trigger",
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1("reply-check-source", "reply-check-type", nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerName),
		)
		client.WaitForResourceReadyOrFail(replyTrigger.Name, testlib.TriggerTypeMeta)

		st.Run("No upgrade of version", func(t *testing.T) {
			event := baseEvent
			source := "no-upgrade"
			event.SetID(source)
			event.Context = event.Context.AsV03()
			senderName := source + "-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			originalEventMatcher := recordevents.MatchEvent(cetest.AllOf(
				cetest.HasSpecVersion("0.3"),
				cetest.HasId("no-upgrade"),
			))
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertExact(1, originalEventMatcher)

		})

		st.Run("Attributes received should be the same as produced (attributes may be added)", func(t *testing.T) {
			event := baseEvent
			id := "identical-attributes"
			event.SetID(id)
			senderName := id + "-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			originalEventMatcher := recordevents.MatchEvent(
				cetest.HasId(id),
				cetest.HasType(testlib.DefaultEventType),
				cetest.HasSource(baseSource),
				cetest.HasSpecVersion("1.0"),
			)
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertExact(1, originalEventMatcher)
		})

		st.Run("Events are filtered", func(t *testing.T) {
			event := baseEvent
			source := "filtered-event"
			event.SetSource(source)
			secondEvent := baseEvent
			firstSenderName := "first-" + source + "-sender"
			secondSenderName := "second-" + source + "-sender"
			client.SendEventToAddressable(ctx, firstSenderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			client.SendEventToAddressable(ctx, secondSenderName, broker.Name, testlib.BrokerTypeMeta, secondEvent, sender.WithEncoding(ce.EncodingStructured))
			filteredEventMatcher := recordevents.MatchEvent(
				cetest.HasSource(source),
			)
			nonEventMatcher := recordevents.MatchEvent(
				cetest.HasSource(baseSource),
			)
			client.WaitForResourceReadyOrFail(firstSenderName, &podMeta)
			client.WaitForResourceReadyOrFail(secondSenderName, &podMeta)
			secondTracker.AssertAtLeast(1, filteredEventMatcher)
			secondTracker.AssertNot(nonEventMatcher)
		})

		st.Run("Events are delivered to multiple subscribers", func(t *testing.T) {
			event := baseEvent
			source := "filtered-event"
			event.SetSource(source)
			senderName := source + "-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			filteredEventMatcher := recordevents.MatchEvent(
				cetest.HasSource(source),
			)
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertAtLeast(1, filteredEventMatcher)
			secondTracker.AssertAtLeast(1, filteredEventMatcher)
		})

		st.Run("Deliveries succeed at least once", func(t *testing.T) {
			event := baseEvent
			source := "delivery-check"
			event.SetSource(source)
			senderName := source + "-sender"
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			originalEventMatcher := recordevents.MatchEvent(
				cetest.HasSource(source),
			)
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertAtLeast(1, originalEventMatcher)
		})

		st.Run("Replies are accepted and delivered", func(t *testing.T) {
			event := baseEvent
			event.SetSource(replySource)
			senderName := replySource + "-sender"
			client.WaitForServiceEndpointsOrFail(ctx, transformerName, 1)
			client.WaitForResourceReadyOrFail(transformTrigger.Name, testlib.TriggerTypeMeta)
			client.WaitForResourceReadyOrFail(replyTrigger.Name, testlib.TriggerTypeMeta)
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.WithEncoding(ce.EncodingStructured))
			transformedEventMatcher := recordevents.MatchEvent(
				cetest.HasSource("reply-check-source"),
				cetest.HasType("reply-check-type"),
				cetest.HasData(transformMsg),
			)
			client.WaitForResourceReadyOrFail(senderName, &podMeta)
			eventTracker.AssertAtLeast(2, transformedEventMatcher)
		})
	})
}
