// +build e2e

/*
Copyright 2019 The Knative Authors
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

package e2e

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	pkgResources "knative.dev/eventing/pkg/reconciler/namespace/resources"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

func TestApiServerSource(t *testing.T) {
	const (
		baseApiServerSourceName = "e2e-api-server-source"

		roleName              = "event-watcher-r"
		serviceAccountName    = "event-watcher-sa"
		baseHelloworldPodName = "e2e-api-server-source-helloworld-pod"
		baseLoggerPodName     = "e2e-api-server-source-logger-pod"
	)

	mode := "Reference"
	table := []struct {
		name     string
		spec     sourcesv1alpha2.ApiServerSourceSpec
		pod      func(name string) *corev1.Pod
		expected string
	}{
		{
			name: "event-ref",
			spec: sourcesv1alpha2.ApiServerSourceSpec{
				Resources: []sourcesv1alpha2.APIVersionKindSelector{{
					APIVersion: ptr.String("v1"),
					Kind:       ptr.String("Event"),
				}},
				EventMode:          mode,
				ServiceAccountName: serviceAccountName,
			},
			pod:      func(name string) *corev1.Pod { return resources.HelloWorldPod(name) },
			expected: "Event",
		},
		{
			name: "event-ref-unmatch-label",
			spec: sourcesv1alpha2.ApiServerSourceSpec{
				Resources: []sourcesv1alpha2.APIVersionKindSelector{{
					APIVersion:    ptr.String("v1"),
					Kind:          ptr.String("Pod"),
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
				}},
				EventMode:          mode,
				ServiceAccountName: serviceAccountName,
			},
			pod:      func(name string) *corev1.Pod { return resources.HelloWorldPod(name) },
			expected: "",
		},
		{
			name: "event-ref-match-label",
			spec: sourcesv1alpha2.ApiServerSourceSpec{
				Resources: []sourcesv1alpha2.APIVersionKindSelector{{
					APIVersion:    ptr.String("v1"),
					Kind:          ptr.String("Pod"),
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
				}},
				EventMode:          mode,
				ServiceAccountName: serviceAccountName,
			},
			pod: func(name string) *corev1.Pod {
				return resources.HelloWorldPod(name, resources.WithLabelsForPod(map[string]string{"e2e": "testing"}))
			},
			expected: "Pod",
		},
		{
			name: "event-ref-match-label-expr",
			spec: sourcesv1alpha2.ApiServerSourceSpec{
				Resources: []sourcesv1alpha2.APIVersionKindSelector{{
					APIVersion: ptr.String("v1"),
					Kind:       ptr.String("Pod"),
					LabelSelector: &metav1.LabelSelector{
						MatchLabels:      map[string]string{"e2e": "testing"},
						MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "e2e", Operator: "Exists"}},
					},
				}},
				EventMode:          mode,
				ServiceAccountName: serviceAccountName,
			},
			pod: func(name string) *corev1.Pod {
				return resources.HelloWorldPod(name, resources.WithLabelsForPod(map[string]string{"e2e": "testing"}))
			},
			expected: "Pod",
		},
	}

	client := setup(t, true)
	defer tearDown(client)

	// creates ServiceAccount and RoleBinding with a role for reading pods and events
	r := resources.Role(roleName,
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events", "pods"},
			Verbs:     []string{"get", "list", "watch"}}))
	client.CreateServiceAccountOrFail(serviceAccountName)
	client.CreateRoleOrFail(r)
	client.CreateRoleBindingOrFail(
		serviceAccountName,
		lib.RoleKind,
		roleName,
		fmt.Sprintf("%s-%s", serviceAccountName, roleName),
		client.Namespace,
	)

	for _, tc := range table {

		// create event logger pod and service
		loggerPodName := fmt.Sprintf("%s-%s", baseLoggerPodName, tc.name)
		tc.spec.Sink = duckv1.Destination{Ref: resources.ServiceKRef(loggerPodName)}

		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

		apiServerSource := eventingtesting.NewApiServerSource(
			fmt.Sprintf("%s-%s", baseApiServerSourceName, tc.name),
			client.Namespace,
			eventingtesting.WithApiServerSourceSpec(tc.spec),
		)

		client.CreateApiServerSourceOrFail(apiServerSource)

		// wait for all test resources to be ready
		client.WaitForAllTestResourcesReadyOrFail()

		helloworldPod := tc.pod(fmt.Sprintf("%s-%s", baseHelloworldPodName, tc.name))
		client.CreatePodOrFail(helloworldPod)

		// verify the logger service receives the event(s)
		// TODO(chizhg): right now it's only doing a very basic check by looking for the tc.data word,
		//                we can add a json matcher to improve it in the future.

		if tc.expected == "" {
			if err := client.CheckLogEmpty(loggerPodName, 10*time.Second); err != nil {
				t.Fatalf("Log is not empty in logger pod %q: %v", loggerPodName, err)
			}

		} else {
			if err := client.CheckLog(loggerPodName, lib.CheckerContains(tc.expected)); err != nil {
				t.Fatalf("String %q does not appear in logs of logger pod %q: %v", tc.expected, loggerPodName, err)
			}
		}
	}
}

func TestApiServerSourceV1Alpha2EventTypes(t *testing.T) {
	const (
		sourceName         = "e2e-apiserver-source-eventtypes"
		serviceAccountName = "event-watcher-sa"
		roleName           = "event-watcher-r"
	)

	client := setup(t, true)
	defer tearDown(client)

	// creates ServiceAccount and RoleBinding with a role for reading pods and events
	r := resources.Role(roleName,
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events", "pods"},
			Verbs:     []string{"get", "list", "watch"}}))
	client.CreateServiceAccountOrFail(serviceAccountName)
	client.CreateRoleOrFail(r)
	client.CreateRoleBindingOrFail(
		serviceAccountName,
		lib.RoleKind,
		roleName,
		fmt.Sprintf("%s-%s", serviceAccountName, roleName),
		client.Namespace,
	)

	// Label namespace so that it creates the default broker.
	if err := client.LabelNamespace(map[string]string{"knative-eventing-injection": "enabled"}); err != nil {
		t.Fatalf("Error annotating namespace: %v", err)
	}

	// Wait for default broker ready.
	client.WaitForResourceReadyOrFail(pkgResources.DefaultBrokerName, lib.BrokerTypeMeta)

	// Create the api server source
	apiServerSource := eventingtesting.NewApiServerSource(
		sourceName,
		client.Namespace,
		eventingtesting.WithApiServerSourceSpec(
			sourcesv1alpha2.ApiServerSourceSpec{
				Resources: []sourcesv1alpha2.APIVersionKindSelector{{
					APIVersion: ptr.String("v1"),
					Kind:       ptr.String("Event"),
				}},
				EventMode:          "Reference",
				ServiceAccountName: serviceAccountName,
				// TODO change sink to be a non-Broker one once we revisit EventType https://github.com/knative/eventing/issues/2750
			}),
	)
	apiServerSource.Spec.Sink = duckv1.Destination{Ref: &duckv1.KReference{APIVersion: "eventing.knative.dev/v1alpha1", Kind: "Broker", Name: pkgResources.DefaultBrokerName, Namespace: client.Namespace}}

	client.CreateApiServerSourceOrFail(apiServerSource)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail()

	// verify that EventTypes were created.
	eventTypes, err := client.Eventing.EventingV1alpha1().EventTypes(client.Namespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Error retrieving EventTypes: %v", err)
	}
	if len(eventTypes.Items) != len(sourcesv1alpha2.ApiServerSourceEventTypes) {
		t.Fatalf("Invalid number of EventTypes registered for ApiServerSource: %s/%s, expected: %d, got: %d", client.Namespace, sourceName, len(sourcesv1alpha2.ApiServerSourceEventTypes), len(eventTypes.Items))
	}

	expectedCeTypes := sets.NewString(sourcesv1alpha2.ApiServerSourceEventTypes...)
	for _, et := range eventTypes.Items {
		if !expectedCeTypes.Has(et.Spec.Type) {
			t.Fatalf("Invalid spec.type for ApiServerSource EventType, expected one of: %v, got: %s", sourcesv1alpha2.ApiServerSourceEventTypes, et.Spec.Type)
		}
	}
}
