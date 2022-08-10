/*
Copyright 2021 The Knative Authors

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

package apiserversource

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/test"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/eventing/pkg/apis/sources"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/pod"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

const (
	exampleImage = "ko://knative.dev/eventing/test/test_images/print"
)

func DataPlane_SinkTypes() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative ApiServerSource - Data Plane - Sink Types",
		Features: []*feature.Feature{
			SendsEventsWithSinkRef(),
			SendsEventsWithSinkUri(),
			SendsEventsWithEventTypes(),

			// TODO: things to test:
			// - check if we actually receive add, update and delete events
		},
	}

	return fs
}

func DataPlane_EventModes() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative ApiServerSource - Data Plane - Event Modes",
		Features: []*feature.Feature{
			SendsEventsWithObjectReferencePayload(),
			SendsEventsWithResourceEventPayload(),

			// TODO: things to test:
			// - check if we actually receive add, update and delete events
		},
	}

	return fs
}

func DataPlane_ResourceMatching() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative ApiServerSource - Data Plane - Resource Matching",
		Features: []*feature.Feature{
			SendsEventsForAllResources(),
			SendsEventsForLabelMatchingResources(),
			//*DoesNotSendEventsForNonLabelMatchingResources(),
			SendEventsForLabelExpressionMatchingResources(),

			// TODO: things to test:
			// - check if we actually receive add, update and delete events
		},
	}

	return fs
}

func SendsEventsWithSinkRef() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events to sink ref")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithSink(svc.AsKReference(sink), ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with ref",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1))

	return f
}

func SendsEventsWithSinkUri() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events to sink uri")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	f.Setup("install ApiServerSource", func(ctx context.Context, t feature.T) {
		sinkuri, err := svc.Address(ctx, sink)
		if err != nil || sinkuri == nil {
			t.Error("failed to get the address of the sink service", sink, err)
		}

		cfg := []manifest.CfgFn{
			apiserversource.WithServiceAccountName(sacmName),
			apiserversource.WithEventMode(v1.ResourceMode),
			apiserversource.WithSink(nil, sinkuri.String()),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Event",
			}),
		}

		apiserversource.Install(source, cfg...)(ctx, t)
	})
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with URI",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1))

	return f
}

// SendsEventsWithEventTypes tests apiserversource to a ready broker.
func SendsEventsWithEventTypes() *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	f := new(feature.Feature)

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(svc.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))

	f.Setup("trigger goes ready", trigger.IsReady(via))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	f.Setup("install apiserversource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			apiserversource.WithServiceAccountName(sacmName),
			apiserversource.WithEventMode(v1.ResourceMode),
			apiserversource.WithSink(nil, brokeruri.String()),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Event",
			}),
		}
		apiserversource.Install(source, cfg...)(ctx, t)
	})
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	expectedCeTypes := sets.NewString(sources.ApiServerSourceEventReferenceModeTypes...)

	f.Stable("ApiServerSource as event source").
		Must("delivers events on broker with URI",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1)).
		Must("ApiServerSource test eventtypes match",
			eventtype.WaitForEventType(eventtype.AssertPresent(expectedCeTypes)))

	return f
}

func SendsEventsWithObjectReferencePayload() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events with ObjectReference payload")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ReferenceMode),
		apiserversource.WithSink(svc.AsKReference(sink), ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Pod",
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName, pod.WithImage(exampleImage)),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
			).AtLeast(1))

	return f
}

func SendsEventsWithResourceEventPayload() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events with ResourceEvent payload")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithSink(svc.AsKReference(sink), ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Pod",
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName, pod.WithImage(exampleImage)),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.resource.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
			).AtLeast(1))

	return f
}

func SendsEventsForAllResources() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events for all resources")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode("Reference"),
		apiserversource.WithSink(svc.AsKReference(sink), ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Pod",
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName, pod.WithImage(exampleImage)),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
			).AtLeast(1))

	return f
}

func SendsEventsForLabelMatchingResources() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events for label-matching resources")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode("Reference"),
		apiserversource.WithSink(svc.AsKReference(sink), ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion:    "v1",
			Kind:          "Pod",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName,
			pod.WithImage(exampleImage),
			pod.WithLabels(map[string]string{"e2e": "testing"})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.update"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
			).AtLeast(1))

	return f
}

// THIS TEST DOES NOT WORK
//func DoesNotSendEventsForNonLabelMatchingResources() *feature.Feature {
//	source := feature.MakeRandomK8sName("apiserversource")
//	sink := feature.MakeRandomK8sName("sink")
//	f := feature.NewFeatureNamed("Does not send events for label-unmatching resources")
//
//	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
//
//	sacmName := feature.MakeRandomK8sName("apiserversource")
//	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
//		setupAccountAndRoleForPods(sacmName))
//
//	cfg := []manifest.CfgFn{
//		apiserversource.WithServiceAccountName(sacmName),
//		apiserversource.WithEventMode("Reference"),
//		apiserversource.WithSink(svc.AsKReference(sink), ""),
//		apiserversource.WithResources(v1.APIVersionKindSelector{
//			APIVersion:    "v1",
//			Kind:          "Pod",
//			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "something-else"}},
//		}),
//	}
//
//	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
//	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))
//
//	examplePodName := feature.MakeRandomK8sName("example")
//
//	// create a pod so that ApiServerSource delivers an event to its sink
//	// event body is similar to this:
//	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
//	f.Requirement("install example pod",
//		pod.Install(examplePodName,
//			pod.WithImage(exampleImage),
//			pod.WithLabels(map[string]string{"e2e": "testing"})),
//	)
//
//	f.Stable("ApiServerSource as event source").
//		Must("does not deliver events for unmatched resources", func(ctx context.Context, t feature.T) {
//			// sleep some time to make sure the sink doesn't actually receive events
//			// not because reaction time was too short.
//			time.Sleep(10 * time.Second)
//			eventasssert.OnStore(sink).MatchEvent(any()).Not()(ctx, t)
//		})
//
//	return f
//}

func SendEventsForLabelExpressionMatchingResources() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events for label-expression-matching resources")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode("Reference"),
		apiserversource.WithSink(svc.AsKReference(sink), ""),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion:    "v1",
			Kind:          "Pod",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "e2e", Operator: "Exists"}}},
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName,
			pod.WithImage(exampleImage),
			pod.WithLabels(map[string]string{"e2e": "testing"})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
			).AtLeast(1))

	return f
}

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

//// any matches any event
//func any() test.EventMatcher {
//	return func(have cloudevent.Event) error {
//		return nil
//	}
//}

func SendsEventsWithRetries() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")

	f := feature.NewFeatureNamed("Send events with retries")

	// drop first event to see the retry feature works or not
	f.Setup("install sink",
		eventshub.Install(sink,
			eventshub.StartReceiver,
			eventshub.DropFirstN(1),
			eventshub.DropEventsResponseCode(429),
		),
	)

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	f.Setup("install ApiServerSource", func(ctx context.Context, t feature.T) {
		sinkuri, err := svc.Address(ctx, sink)
		if err != nil || sinkuri == nil {
			t.Fatal("failed to get the address of the sink service", sink, err)
		}

		cfg := []manifest.CfgFn{
			apiserversource.WithServiceAccountName(sacmName),
			apiserversource.WithEventMode(v1.ReferenceMode),
			apiserversource.WithSink(nil, sinkuri.String()),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion:    "v1",
				Kind:          "Pod",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
			}),
		}
		apiserversource.Install(source, cfg...)(ctx, t)
	})
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName,
			pod.WithImage(exampleImage),
			pod.WithLabels(map[string]string{"e2e": "testing"})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventasssert.OnStore(sink).Match(
				eventasssert.MatchKind(eventasssert.EventReceived),
				eventasssert.MatchEvent(
					test.HasType("dev.knative.apiserver.ref.add"),
					test.DataContains(`"kind":"Pod"`),
					test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
				),
			).AtLeast(1))
	return f
}
