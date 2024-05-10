/*
Copyright 2024 The Knative Authors

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

package apiserversource_newfilters

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"knative.dev/eventing/test/rekt/resources/account_role"

	"github.com/cloudevents/sdk-go/v2/test"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/pod"
	"knative.dev/reconciler-test/pkg/resources/service"
)

const (
	exampleImage = "ko://knative.dev/eventing/test/test_images/print"
)

func NewFiltersFeature() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative ApiServerSource - Features - New Filter",
		Features: []*feature.Feature{
			EventsAreFilteredOut(),
		},
	}
	return fs
}

func EventsAreFilteredOut() *feature.Feature {
	source := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeatureNamed("Filters properly the messages")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	sacmName := feature.MakeRandomK8sName("apiserversource")
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Pod resources",
		setupAccountAndRoleForPods(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithSink(service.AsDestinationRef(sink)),
		apiserversource.WithFilters([]eventingv1.SubscriptionsAPIFilter{{
			Exact: map[string]string{
				"type": "dev.knative.apiserver.resource.update",
			},
		}}),
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
			).Exact(0)).
		Must("delivers events",
			eventassert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.apiserver.resource.update"),
				test.HasExtensions(map[string]interface{}{"apiversion": "v1"}),
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
