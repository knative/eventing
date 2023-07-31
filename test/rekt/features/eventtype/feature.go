/*
Copyright 2022 The Knative Authors

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

package eventtype

import (
	"context"
	"github.com/cloudevents/sdk-go/v2/test"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/eventing/pkg/apis/sources"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func setupAccountAndRoleForPods(sacmName string) feature.StepFn {
	return account_role.Install(sacmName,
		account_role.WithRole(sacmName+"-clusterrole"),
		account_role.WithRules(rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		}),
	)
}

func EventTypeWithBrokerAsReference() *feature.Feature {
	//f := feature.NewFeatureNamed("Broker reply with a bad status code to the first n events")
	//
	//source := feature.MakeRandomK8sName("source")
	//sink := feature.MakeRandomK8sName("sink")
	//via := feature.MakeRandomK8sName("via")
	//
	//eventSource := "source2"
	//eventType := "type2"
	//eventBody := `{"msg":"DropN"}`
	//event := cloudevents.NewEvent()
	//event.SetID(uuid.New().String())
	//event.SetType(eventType)
	//event.SetSource(eventSource)
	//event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))
	//
	////Install the broker
	//brokerName := feature.MakeRandomK8sName("broker")
	//
	//f.Setup("install broker", broker.Install(brokerName))
	//f.Requirement("broker is ready", broker.IsReady(brokerName))
	//f.Requirement("broker is addressable", broker.IsAddressable(brokerName))
	//
	//f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	//
	//// Point the Trigger subscriber to the sink svc.
	//cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), "")}
	//
	//// Install the trigger
	//f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))
	//f.Setup("trigger goes ready", trigger.IsReady(via))
	//
	//f.Requirement("install source", eventshub.Install(
	//	source,
	//	eventshub.StartSenderToResource(broker.GVR(), brokerName),
	//	eventshub.InputEvent(event),
	//))
	//
	//f.Stable("containersource as event source").
	//	Must("delivers events",
	//		assert.OnStore(sink).
	//			Match(assert.MatchKind(eventshub.EventReceived)).
	//			MatchEvent(test.HasType("type3")).
	//			AtLeast(1),
	//	).Must("ApiServerSource test eventtypes match",
	//	eventtype.WaitForEventType(eventtype.AssertReferenceMatch("InMemoryChannel")))
	//
	//// The eventType should be already auto-created. We then need to pop up the eventType in the event registry.
	//// We need to validate the reference of the eventType is pointing to the broker.

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	f := new(feature.Feature)

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install trigger", trigger.Install(via, brokerName, trigger.WithSubscriber(service.AsKReference(sink), "")))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	f.Requirement("install apiserversource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			apiserversource.WithServiceAccountName(sacmName),
			apiserversource.WithEventMode(v1.ResourceMode),
			apiserversource.WithSink(&duckv1.Destination{URI: brokeruri.URL, CACerts: brokeruri.CACerts}),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Event",
			}),
		}
		apiserversource.Install(source, cfg...)(ctx, t)
	})
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	expectedCeTypes := sets.NewString(sources.ApiServerSourceEventReferenceModeTypes...)

	f.Stable("ApiServerSource as event source").
		Must("delivers events on broker with URI",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1)).
		Must("ApiServerSource test eventtypes match",
			eventtype.WaitForEventType(eventtype.AssertPresent(expectedCeTypes)))

	return f

}
