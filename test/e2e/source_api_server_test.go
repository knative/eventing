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
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"

	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestApiServerSource(t *testing.T) {
	const (
		baseApiServerSourceName = "e2e-api-server-source"

		clusterRoleName       = "event-watcher-cr"
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
						APIVersion:    "v1",
						Kind:          "Event",
						LabelSelector: &metav1.LabelSelector{},
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
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
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
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
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
						LabelSelector: &metav1.LabelSelector{
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

	// creates ServiceAccount and ClusterRoleBinding with default cluster-admin role
	cr := resources.ClusterRole(clusterRoleName,
		resources.WithRuleForClusterRole(&rbacv1.PolicyRule{
			APIGroups: []string{rbacv1.APIGroupAll},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"}}),
		resources.WithRuleForClusterRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch"}}))
	client.CreateServiceAccountOrFail(serviceAccountName)
	client.CreateClusterRoleOrFail(cr)
	client.CreateClusterRoleBindingOrFail(
		serviceAccountName,
		clusterRoleName,
		fmt.Sprintf("%s-%s", serviceAccountName, clusterRoleName),
	)

	for _, tc := range table {

		// create event logger pod and service
		loggerPodName := fmt.Sprintf("%s-%s", baseLoggerPodName, tc.name)
		tc.spec.Sink = resources.ServiceRef(loggerPodName)

		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

		apiServerSource := eventingtesting.NewApiServerSource(
			fmt.Sprintf("%s-%s", baseApiServerSourceName, tc.name),
			client.Namespace,
			eventingtesting.WithApiServerSourceSpec(tc.spec),
		)

		client.CreateApiServerSourceOrFail(apiServerSource)

		// wait for all test resources to be ready
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			t.Fatalf("Failed to get all test resources ready: %v", err)
		}

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
			if err := client.CheckLog(loggerPodName, common.CheckerContains(tc.expected)); err != nil {
				t.Fatalf("String %q does not appear in logs of logger pod %q: %v", tc.expected, loggerPodName, err)
			}
		}
	}
}
