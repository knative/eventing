//go:build e2e
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
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"

	"knative.dev/eventing/pkg/reconciler/sugar"

	sugarresources "knative.dev/eventing/pkg/reconciler/sugar/resources"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis/sources"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	rttestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

func TestApiServerSourceV1(t *testing.T) {
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
		spec     sourcesv1.ApiServerSourceSpec
		pod      func(name string) *corev1.Pod
		expected string
	}{
		{
			name: "event-ref",
			spec: sourcesv1.ApiServerSourceSpec{
				Resources: []sourcesv1.APIVersionKindSelector{{
					APIVersion: "v1",
					Kind:       "Event",
				}},
				EventMode:          mode,
				ServiceAccountName: serviceAccountName,
			},
			pod:      func(name string) *corev1.Pod { return resources.HelloWorldPod(name) },
			expected: "Event",
		},
		{
			name: "event-ref-unmatch-label",
			spec: sourcesv1.ApiServerSourceSpec{
				Resources: []sourcesv1.APIVersionKindSelector{{
					APIVersion:    "v1",
					Kind:          "Pod",
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"e2e": "testing"}},
				}},
				EventMode:          mode,
				ServiceAccountName: serviceAccountName,
			},
			pod: func(name string) *corev1.Pod { return resources.HelloWorldPod(name) },
		},
		{
			name: "event-ref-match-label",
			spec: sourcesv1.ApiServerSourceSpec{
				Resources: []sourcesv1.APIVersionKindSelector{{
					APIVersion:    "v1",
					Kind:          "Pod",
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
			spec: sourcesv1.ApiServerSourceSpec{
				Resources: []sourcesv1.APIVersionKindSelector{{
					APIVersion: "v1",
					Kind:       "Pod",
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

	ctx := context.Background()

	for _, tc := range table {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// Setup client
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
				testlib.RoleKind,
				roleName,
				fmt.Sprintf("%s-%s", serviceAccountName, roleName),
				client.Namespace,
			)

			// create event record
			recordEventPodName := fmt.Sprintf("%s-%s", baseLoggerPodName, tc.name)
			eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventPodName)
			spec := tc.spec
			spec.Sink = duckv1.Destination{Ref: resources.ServiceKRef(recordEventPodName)}

			apiServerSource := rttestingv1.NewApiServerSource(
				fmt.Sprintf("%s-%s", baseApiServerSourceName, tc.name),
				client.Namespace,
				rttestingv1.WithApiServerSourceSpec(spec),
			)

			client.CreateApiServerSourceV1OrFail(apiServerSource)

			// wait for all test resources to be ready
			client.WaitForAllTestResourcesReadyOrFail(ctx)

			helloworldPod := tc.pod(fmt.Sprintf("%s-%s", baseHelloworldPodName, tc.name))
			client.CreatePodOrFail(helloworldPod)

			// verify the logger service receives the event(s)
			// TODO(chizhg): right now it's only doing a very basic check by looking for the tc.data word,
			//                we can add a json matcher to improve it in the future.

			// Run asserts
			if tc.expected == "" {
				time.Sleep(10 * time.Second)
				eventTracker.AssertNot(recordevents.Any())
			} else {
				eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
					cetest.DataContains(tc.expected),
				))
			}
		})
	}
}

func TestApiServerSourceV1EventTypes(t *testing.T) {
	const (
		sourceName         = "e2e-apiserver-source-eventtypes"
		serviceAccountName = "event-watcher-sa"
		roleName           = "event-watcher-r"
	)

	client := setup(t, true)
	defer tearDown(client)

	ctx := context.Background()

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
		testlib.RoleKind,
		roleName,
		fmt.Sprintf("%s-%s", serviceAccountName, roleName),
		client.Namespace,
	)

	// Label namespace so that it creates the default broker.
	if err := client.LabelNamespace(map[string]string{sugar.InjectionLabelKey: sugar.InjectionEnabledLabelValue}); err != nil {
		t.Fatalf("Error annotating namespace: %v", err)
	}

	// Wait for default broker ready.
	client.WaitForResourceReadyOrFail(sugarresources.DefaultBrokerName, testlib.BrokerTypeMeta)

	// Create the api server source
	apiServerSource := rttestingv1.NewApiServerSource(
		sourceName,
		client.Namespace,
		rttestingv1.WithApiServerSourceSpec(
			sourcesv1.ApiServerSourceSpec{
				Resources: []sourcesv1.APIVersionKindSelector{{
					APIVersion: "v1",
					Kind:       "Event",
				}},
				EventMode:          "Reference",
				ServiceAccountName: serviceAccountName,
				// TODO change sink to be a non-Broker one once we revisit EventType https://github.com/knative/eventing/issues/2750
			}),
	)
	apiServerSource.Spec.Sink = duckv1.Destination{Ref: &duckv1.KReference{APIVersion: "eventing.knative.dev/v1", Kind: "Broker", Name: sugarresources.DefaultBrokerName, Namespace: client.Namespace}}

	client.CreateApiServerSourceV1OrFail(apiServerSource)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	eventTypes, err := waitForEventTypes(ctx, client, len(sources.ApiServerSourceEventReferenceModeTypes))
	if err != nil {
		t.Fatalf("Waiting for EventTypes: %v", err)
	}
	expectedCeTypes := sets.NewString(sources.ApiServerSourceEventReferenceModeTypes...)
	for _, et := range eventTypes {
		if !expectedCeTypes.Has(et.Spec.Type) {
			t.Fatalf("Invalid spec.type for ApiServerSource EventType, expected one of: %v, got: %s", sources.ApiServerSourceEventReferenceModeTypes, et.Spec.Type)
		}
	}
}

// waitForEventTypes waits for the expected number of EventTypes to exist in client.Namespace.
func waitForEventTypes(ctx context.Context, client *testlib.Client, expectedNumEventTypes int) ([]v1beta1.EventType, error) {
	eventTypes := &v1beta1.EventTypeList{}
	// Interval and timeout were chosen arbitrarily.
	err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		var err error
		eventTypes, err = client.Eventing.EventingV1beta1().EventTypes(client.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, fmt.Errorf("error listing EventTypes: %w", err)
		}
		if len(eventTypes.Items) == expectedNumEventTypes {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return []v1beta1.EventType{}, fmt.Errorf("error polling for EventTypes: %w", err)
	}
	if actual := len(eventTypes.Items); actual != expectedNumEventTypes {
		return []v1beta1.EventType{}, fmt.Errorf("invalid number of EventTypes registered, expected: %d, got: %d", expectedNumEventTypes, actual)
	}
	return eventTypes.Items, nil
}
