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
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	"knative.dev/eventing/test/rekt/resources/pod"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

func DataPlane() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative ApiServerSource - Data Plane",
		Features: []feature.Feature{
			*SendsEventsWithSinkRef(),
			*SendsEventsWithSinkUri(),

			// TODO: things to test:
			// - payload: ObjectReference vs ResourceEvent
			// - label matching: none VS label matching
			// - sink: as ref VS as uri
			// -  label unmatching

			*SendsEventsWithObjectReferencePayload(),
			*SendsEventsWithResourceEventPayload(),
		},
	}

	addSendEventsMatrix(fs)

	return fs
}

func SendsEventsWithSinkRef() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events to sink ref")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource",
		account_role.Install(sacmName,
			account_role.WithRole(sacmName+"-clusterrole"),
			account_role.WithRules(rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			}),
		))

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
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with ref",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.add")).AtLeast(1))

	return f
}

func SendsEventsWithSinkUri() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events to sink uri")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource",
		account_role.Install(sacmName,
			account_role.WithRole(sacmName+"-clusterrole"),
			account_role.WithRules(rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			}),
		))

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
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with URI",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.add")).AtLeast(1))

	return f
}

func SendsEventsWithObjectReferencePayload() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Send events with object reference payload")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource",
		account_role.Install(sacmName,
			account_role.WithRole(sacmName+"-clusterrole"),
			account_role.WithRules(rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			}),
		))

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
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName, pod.WithImage("ko://knative.dev/eventing/test/test_images/print")),
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
	f := feature.NewFeatureNamed("Send events with object reference payload")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource",
		account_role.Install(sacmName,
			account_role.WithRole(sacmName+"-clusterrole"),
			account_role.WithRules(rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			}),
		))

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
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

	examplePodName := feature.MakeRandomK8sName("example")

	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod",
		pod.Install(examplePodName, pod.WithImage("ko://knative.dev/eventing/test/test_images/print")),
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

func addSendEventsMatrix(fs *feature.FeatureSet) {
	sinkCases := []struct {
		name  string
		cfgFn func(ctx context.Context, t feature.T, sinkName string) manifest.CfgFn
	}{{
		name: "sink ref",
		cfgFn: func(_ context.Context, _ feature.T, sinkName string) manifest.CfgFn {
			return apiserversource.WithSink(svc.AsKReference(sinkName), "")
		},
	}, {
		name: "sink uri",
		cfgFn: func(ctx context.Context, t feature.T, sinkName string) manifest.CfgFn {
			if uri, err := svc.Address(ctx, sinkName); err == nil {
				return apiserversource.WithSink(nil, uri.String())
			} else {
				t.Errorf("Unable to get address of sink: %s. Error: %w", sinkName, err)
				return nil
			}
		},
	}}

	eventModeCases := []struct {
		mode              string
		expectedEventType string
	}{{
		mode:              "Reference",
		expectedEventType: "dev.knative.apiserver.ref.add",
	}, {
		mode:              "Resource",
		expectedEventType: "dev.knative.apiserver.resource.add",
	}}

	resourceMatchCases := []struct {
		name      string
		resources []v1.APIVersionKindSelector
		pod       func() []manifest.CfgFn
		matchers  []test.EventMatcher
	}{{
		name: "all pods",
		resources: []v1.APIVersionKindSelector{{
			APIVersion: "v1",
			Kind:       "Pod",
		}},
		pod: func() []manifest.CfgFn {
			return []manifest.CfgFn{
				pod.WithImage("ko://knative.dev/eventing/test/test_images/print"),
			}
		}}, {
		name: "pods matching labels",
		resources: []v1.APIVersionKindSelector{{
			APIVersion:    "v1",
			Kind:          "Pod",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
		}},
		pod: func() []manifest.CfgFn {
			return []manifest.CfgFn{
				pod.WithImage("ko://knative.dev/eventing/test/test_images/print"),
				pod.WithLabels(map[string]string{"e2e": "testing"}),
			}
		}}, {
		name: "pods matching label expressions",
		resources: []v1.APIVersionKindSelector{{
			APIVersion:    "v1",
			Kind:          "Pod",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "e2e", Operator: "Exists"}}},
		}},
		pod: func() []manifest.CfgFn {
			return []manifest.CfgFn{
				pod.WithImage("ko://knative.dev/eventing/test/test_images/print"),
				pod.WithLabels(map[string]string{"e2e": "testing"}),
			}
		}},
	}

	for _, sinkCase := range sinkCases {
		for _, eventModeCase := range eventModeCases {
			for _, resourceMatchCase := range resourceMatchCases {
				source := feature.MakeRandomK8sName("apiserversource")
				sink := feature.MakeRandomK8sName("sink")
				f := feature.NewFeatureNamed(fmt.Sprintf("Delivers events for %s to %s with event payload of %s", resourceMatchCase.name, sinkCase.name, eventModeCase))

				f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

				sacmName := feature.MakeRandomK8sName("apiserversource")
				f.Setup("Create Service Account for ApiServerSource",
					account_role.Install(sacmName,
						account_role.WithRole(sacmName+"-clusterrole"),
						account_role.WithRules(rbacv1.PolicyRule{
							APIGroups: []string{""},
							Resources: []string{"events"},
							Verbs:     []string{"get", "list", "watch"},
						}),
					))

				f.Setup("install ApiServerSource", func(ctx context.Context, t feature.T) {
					cfg := []manifest.CfgFn{
						apiserversource.WithServiceAccountName(sacmName),
						apiserversource.WithEventMode(eventModeCase.mode),
						sinkCase.cfgFn(ctx, t, sink),
						apiserversource.WithResources(resourceMatchCase.resources...),
					}

					apiserversource.Install(source, cfg...)(ctx, t)
				})
				f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(source))

				examplePodName := feature.MakeRandomK8sName("example")

				// create a pod so that ApiServerSource delivers an event to its sink
				// event body is similar to this:
				// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
				f.Requirement("install example pod",
					pod.Install(examplePodName, resourceMatchCase.pod()...),
				)

				f.Stable("ApiServerSource as event source").
					Must("delivers events",
						eventasssert.OnStore(sink).MatchEvent(
							test.HasType(eventModeCase.expectedEventType),
							test.DataContains(`"kind":"Pod"`),
							test.DataContains(fmt.Sprintf(`"name":"%s"`, examplePodName))).AtLeast(1))

				fs.Features = append(fs.Features, *f)
			}
		}
	}
}

// any matches any event
func any() test.EventMatcher {
	return nil
}
