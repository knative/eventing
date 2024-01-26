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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/reconciler-test/pkg/environment"

	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/test/rekt/resources/addressable"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"

	"knative.dev/reconciler-test/pkg/resources/pod"

	"knative.dev/eventing/pkg/apis/sources"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/features/source"
	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/namespace"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/trigger"
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
	f.Requirement("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithSink(service.AsDestinationRef(sink)),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}),
	}

	f.Requirement("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with ref",
			eventassert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1))

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

	f.Requirement("install ApiServerSource", func(ctx context.Context, t feature.T) {
		sinkuri, err := service.Address(ctx, sink)
		if err != nil || sinkuri == nil {
			t.Error("failed to get the address of the sink service", sink, err)
		}

		cfg := []manifest.CfgFn{
			apiserversource.WithServiceAccountName(sacmName),
			apiserversource.WithEventMode(v1.ResourceMode),
			apiserversource.WithSink(&duckv1.Destination{URI: sinkuri.URL, CACerts: sinkuri.CACerts}),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Event",
			}),
		}

		apiserversource.Install(source, cfg...)(ctx, t)
	})
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with URI",
			eventassert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1))

	return f
}

func SendsEventsWithTLS() *feature.Feature {
	src := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")

	f := feature.NewFeatureNamed("Send events to TLS sink")

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Requirement("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}),
	}

	f.Requirement("install ApiServerSource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)

		cfg = append(cfg, apiserversource.WithSink(d))
		apiserversource.Install(src, cfg...)(ctx, t)
	})
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(src))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with ref",
			eventassert.OnStore(sink).
				Match(eventassert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(apiserversource.Gvr(), src)).
		Must("Set sinkCACerts to non empty CA certs", source.ExpectCACerts(apiserversource.Gvr(), src))

	return f
}

func SendsEventsWithTLSTrustBundle() *feature.Feature {
	src := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")

	f := feature.NewFeatureNamed("Send events to TLS sink - trust bundle")

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Requirement("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}),
	}

	f.Requirement("install ApiServerSource", func(ctx context.Context, t feature.T) {
		cfg = append(cfg, apiserversource.WithSink(&duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sink, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}))
		apiserversource.Install(src, cfg...)(ctx, t)
	})
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(src))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with ref",
			eventassert.OnStore(sink).
				Match(eventassert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(apiserversource.Gvr(), src))

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

	expectedCeTypes := sets.New(sources.ApiServerSourceEventResourceModeTypes...)

	f.Stable("ApiServerSource as event source").
		Must("delivers events on broker with URI",
			eventassert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1)).
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
		apiserversource.WithSink(service.AsDestinationRef(sink)),
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
	f.Requirement("install example pod", pod.Install(examplePodName, exampleImage))

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.HasExtensions(map[string]interface{}{"apiversion": "v1"}),
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
		apiserversource.WithSink(service.AsDestinationRef(sink)),
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
	f.Requirement("install example pod", pod.Install(examplePodName, exampleImage))

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.resource.add"),
				test.HasExtensions(map[string]interface{}{"apiversion": "v1"}),
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
		apiserversource.WithSink(service.AsDestinationRef(sink)),
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
	f.Requirement("install example pod", pod.Install(examplePodName, exampleImage))

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
			).AtLeast(1))

	return f
}

func SendsEventsForAllResourcesWithNamespaceSelector() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events for select resources within multiple namespaces")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	testNS1 := feature.MakeRandomK8sName("source-namespace-1")
	testNS2 := feature.MakeRandomK8sName("source-namespace-2")
	testNS3 := feature.MakeRandomK8sName("source-namespace-3")

	// create two new namespaces with matching selector
	f.Setup("create a namespace with matching label", namespace.Install(testNS1, namespace.WithLabels(map[string]string{"env": "development"})))
	f.Setup("create a namespace with matching label", namespace.Install(testNS2, namespace.WithLabels(map[string]string{"env": "development"})))

	// create one new namespace that doesn't match selector
	f.Setup("create a namespace without matching label", namespace.Install(testNS3, namespace.WithLabels(map[string]string{"env": "production"})))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode("Reference"),
		apiserversource.WithSink(service.AsDestinationRef(sink)),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Pod",
		}),
		apiserversource.WithNamespaceSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "development"},
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	pod1 := feature.MakeRandomK8sName("example-pod-1")
	pod2 := feature.MakeRandomK8sName("example-pod-2")
	pod3 := feature.MakeRandomK8sName("example-pod-3")
	f.Requirement("install example pod 1", pod.Install(pod1, exampleImage,
		pod.WithNamespace(testNS1)))
	f.Requirement("install example pod 2", pod.Install(pod2, exampleImage,
		pod.WithNamespace(testNS2)))
	f.Requirement("install example pod 3", pod.Install(pod3, exampleImage,
		pod.WithNamespace(testNS3)))

	f.Stable("ApiServerSource as event source").
		Must("delivers events from matching namespace",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, pod1)),
			).AtLeast(1))
	f.Stable("ApiServerSource as event source").
		Must("delivers events from matching namespace",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, pod2)),
			).AtLeast(1))
	f.Stable("ApiServerSource as event source").
		MustNot("must not deliver events from non-matching namespace",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"Pod"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, pod3)),
			).Not())

	// Delete resources including temporary namespaces
	f.Teardown("Deleting resources", f.DeleteResources)
	return f
}

// SendsEventsForAllResourcesWithEmptyNamespaceSelector tests an APIServerSource with an empty namespace selector
// which will target all namespaces
func SendsEventsForAllResourcesWithEmptyNamespaceSelector() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events for select resources within all namespaces")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for sources.knative.dev/v1 PingSources",
		setupAccountAndRoleForPingSources(sacmName))

	testNS1 := feature.MakeRandomK8sName("source-namespace-1")
	testNS2 := feature.MakeRandomK8sName("source-namespace-2")

	// create two new namespaces
	f.Setup("create a namespace", namespace.Install(testNS1))
	f.Setup("create a namespace", namespace.Install(testNS2))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode("Reference"),
		apiserversource.WithSink(service.AsDestinationRef(sink)),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "sources.knative.dev/v1",
			Kind:       "PingSource",
		}),
		apiserversource.WithNamespaceSelector(&metav1.LabelSelector{
			MatchLabels:      map[string]string{},
			MatchExpressions: []metav1.LabelSelectorRequirement{},
		}),
	}

	f.Setup("install ApiServerSource", apiserversource.Install(source, cfg...))
	f.Setup("ApiServerSource goes ready", apiserversource.IsReady(source))

	pingSource1 := feature.MakeRandomK8sName("ping-source-1")
	pingSource2 := feature.MakeRandomK8sName("ping-source-2")

	f.Requirement("install PingSource 1",
		pingsource.Install(pingSource1, pingsource.WithSink(&duckv1.Destination{URI: apis.HTTP("example.com")})),
	)
	f.Requirement("install PingSource 2",
		pingsource.Install(pingSource2, pingsource.WithSink(&duckv1.Destination{URI: apis.HTTP("example.com")})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events from new namespace",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"PingSource"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, pingSource1)),
			).AtLeast(1))
	f.Stable("ApiServerSource as event source").
		Must("delivers events from new namespace",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.ref.add"),
				test.DataContains(`"kind":"PingSource"`),
				test.DataContains(fmt.Sprintf(`"name":"%s"`, pingSource2)),
			).AtLeast(1))

	// Delete resources including temporary namespaces
	f.Teardown("Deleting resources", f.DeleteResources)
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
		apiserversource.WithSink(service.AsDestinationRef(sink)),
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
		pod.Install(examplePodName, exampleImage,
			pod.WithLabels(map[string]string{"e2e": "testing"})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventassert.OnStore(sink).MatchEvent(
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
//			eventassert.OnStore(sink).MatchEvent(any()).Not()(ctx, t)
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
		apiserversource.WithSink(service.AsDestinationRef(sink)),
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
		pod.Install(examplePodName, exampleImage,
			pod.WithLabels(map[string]string{"e2e": "testing"})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventassert.OnStore(sink).MatchEvent(
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

func setupAccountAndRoleForPingSources(sacmName string) feature.StepFn {
	return account_role.Install(sacmName,
		account_role.WithRole(sacmName+"-clusterrole"),
		account_role.WithRules(rbacv1.PolicyRule{
			APIGroups: []string{"", "sources.knative.dev"},
			Resources: []string{"events", "pingsources"},
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

	f.Requirement("install ApiServerSource", func(ctx context.Context, t feature.T) {
		sinkuri, err := service.Address(ctx, sink)
		if err != nil || sinkuri == nil {
			t.Fatal("failed to get the address of the sink service", sink, err)
		}

		cfg := []manifest.CfgFn{
			apiserversource.WithServiceAccountName(sacmName),
			apiserversource.WithEventMode(v1.ReferenceMode),
			apiserversource.WithSink(&duckv1.Destination{URI: sinkuri.URL, CACerts: sinkuri.CACerts}),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion:    "v1",
				Kind:          "Pod",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
			}),
		}
		apiserversource.Install(source, cfg...)(ctx, t)
	})
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Assert("install example pod",
		pod.Install(examplePodName, exampleImage,
			pod.WithLabels(map[string]string{"e2e": "testing"})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventassert.OnStore(sink).Match(
				eventassert.MatchKind(eventassert.EventReceived),
				eventassert.MatchEvent(
					test.HasType("dev.knative.apiserver.ref.add"),
					test.DataContains(`"kind":"Pod"`),
					test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
				),
			).AtLeast(1))
	return f
}

func SendsEventsWithBrokerAsSinkTLS() *feature.Feature {
	src := feature.MakeRandomK8sName("apiserversource")
	sacmName := feature.MakeRandomK8sName("apiserversource")
	brokerName := feature.MakeRandomK8sName("broker")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	f := feature.NewFeature()

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))
	f.Setup("Broker has HTTPS address", broker.ValidateAddress(brokerName, addressable.AssertHTTPSAddress))

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiverTLS))

	f.Setup("install trigger", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sinkName)
		d.CACerts = eventshub.GetCaCerts(ctx)
		trigger.Install(triggerName, brokerName, trigger.WithSubscriberFromDestination(d))(ctx, t)
	})
	f.Setup("Wait for Trigger to become ready", trigger.IsReady(triggerName))

	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}, v1.APIVersionKindSelector{
			APIVersion:    "v1",
			Kind:          "Pod",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
		}),
	}

	f.Requirement("install ApiServerSource", func(ctx context.Context, t feature.T) {
		d := broker.AsDestinationRef(brokerName)

		brokerAddr, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Fatal("failed to get the address of the broker service", brokerName, err)
		}

		d.CACerts = brokerAddr.CACerts

		cfg = append(cfg, apiserversource.WithSink(d))
		apiserversource.Install(src, cfg...)(ctx, t)
	})

	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(src))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName, exampleImage,
			pod.WithLabels(map[string]string{"e2e": "testing"})),
	)

	f.Stable("ApiServerSource as event source").
		Must("delivers events",
			eventassert.OnStore(sinkName).MatchEvent(
				test.HasType(sources.ApiServerSourceUpdateEventType),
			).AtLeast(1),
		)
	eventassert.MatchEvent(
		test.HasType("dev.knative.apiserver.ref.add"),
		test.DataContains(`"kind":"Pod"`),
		test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName)),
	)

	return f

}
