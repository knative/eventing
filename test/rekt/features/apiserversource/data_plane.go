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
	"fmt"

	"github.com/cloudevents/sdk-go/v2/test"
	rbacv1 "k8s.io/api/rbac/v1"

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
	return &feature.FeatureSet{
		Name: "Knative ApiServerSource - Data Plane",
		Features: []feature.Feature{
			// TODO: get rid of following one
			*SendsEventsWithSinkRef(),

			// TODO: things to test:
			// - payload: ObjectReference vs ResourceEvent
			// - label matching: none VS label matching vs label unmatching
			// - sink: as ref VS as uri

			*SendsEventsWithObjectReferencePayload(),
		},
	}
}

func SendsEventsWithSinkRef() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

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
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.add")).AtLeast(1))

	return f
}

func SendsEventsWithObjectReferencePayload() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

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
