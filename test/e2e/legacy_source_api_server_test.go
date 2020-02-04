// +build e2e

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

package e2e

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestLegacyApiServerSource(t *testing.T) {
	const (
		baseApiServerSourceName = "e2e-api-server-source"

		roleName              = "event-watcher-r"
		serviceAccountName    = "event-watcher-sa"
		baseHelloworldPodName = "e2e-api-server-source-helloworld-pod"
		baseLoggerPodName     = "e2e-api-server-source-logger-pod"
	)

	mode := "Ref"
	table := []struct {
		name     string
		spec     sourcesv1alpha1.ApiServerSourceSpec
		pod      func(name string) *corev1.Pod
		expected string
	}{
		{
			name: "event-ref",
			spec: sourcesv1alpha1.ApiServerSourceSpec{
				Resources: []sourcesv1alpha1.ApiServerResource{
					{
						APIVersion: "v1",
						Kind:       "Event",
					},
				},
				Mode:               mode,
				ServiceAccountName: serviceAccountName,
			},
			pod:      func(name string) *corev1.Pod { return resources.HelloWorldPod(name) },
			expected: "Event",
		},
		{
			name: "event-ref-unmatch-label",
			spec: sourcesv1alpha1.ApiServerSourceSpec{
				Resources: []sourcesv1alpha1.ApiServerResource{
					{
						APIVersion:    "v1",
						Kind:          "Pod",
						LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
					},
				},
				Mode:               mode,
				ServiceAccountName: serviceAccountName,
			},
			pod:      func(name string) *corev1.Pod { return resources.HelloWorldPod(name) },
			expected: "",
		},
		{
			name: "event-ref-match-label",
			spec: sourcesv1alpha1.ApiServerSourceSpec{
				Resources: []sourcesv1alpha1.ApiServerResource{
					{
						APIVersion:    "v1",
						Kind:          "Pod",
						LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
					},
				},
				Mode:               mode,
				ServiceAccountName: serviceAccountName,
			},
			pod: func(name string) *corev1.Pod {
				return resources.HelloWorldPod(name, resources.WithLabelsForPod(map[string]string{"e2e": "testing"}))
			},
			expected: "Pod",
		},
		{
			name: "event-ref-match-label-expr",
			spec: sourcesv1alpha1.ApiServerSourceSpec{
				Resources: []sourcesv1alpha1.ApiServerResource{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"e2e": "testing"},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "e2e", Operator: "Exists"},
							},
						},
					},
				},
				Mode:               mode,
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
		tc.spec.Sink = &duckv1beta1.Destination{Ref: resources.ServiceRef(loggerPodName)}

		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

		apiServerSource := eventingtesting.NewLegacyApiServerSource(
			fmt.Sprintf("%s-%s", baseApiServerSourceName, tc.name),
			client.Namespace,
			eventingtesting.WithLegacyApiServerSourceSpec(tc.spec),
		)

		client.CreateLegacyApiServerSourceOrFail(apiServerSource)

		// wait for all test resources to be ready
		client.WaitForAllTestResourcesReadyOrFail()

		helloworldPod := tc.pod(fmt.Sprintf("%s-%s", baseHelloworldPodName, tc.name))
		client.CreatePodOrFail(helloworldPod)

		// verify the logger service receives the event(s)
		// TODO(Fredy-Z): right now it's only doing a very basic check by looking for the tc.data word,
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
