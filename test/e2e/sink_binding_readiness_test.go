// +build e2e

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

package e2e

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rttestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
)

func TestSinkBindingIsntReadyWithNonExistingSubject(t *testing.T) {
	for _, tc := range sinkBindingIsntReadyWithNonExistingSubjectCases() {
		t.Run(tc.name, func(t *testing.T) {
			runSinkBindingIsntReadyWithNonExistingSubjectCase(t, tc)
		})
	}

}

func runSinkBindingIsntReadyWithNonExistingSubjectCase(
	t *testing.T,
	tc sinkBindingIsntReadyWithNonExistingSubjectCase,
) {
	t.Helper()
	if tc.skippedMsg != "" {
		t.Skip(tc.skippedMsg)
	}
	const (
		sinkBindingName    = "non-existing-subject"
		recordEventPodName = "record-event"
	)
	client := testlib.Setup(t, true)
	defer testlib.TearDown(client)
	ctx := context.Background()

	// create event logger pod and service
	_, _ = recordevents.StartEventRecordOrFail(ctx, client, recordEventPodName)
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	sink := duckv1.Destination{
		Ref: resources.KnativeRefForService(recordEventPodName, client.Namespace),
	}
	subject := tc.subject
	subject.Namespace = client.Namespace
	sinkBinding := rttestingv1.NewSinkBinding(
		sinkBindingName,
		client.Namespace,
		rttestingv1.WithSink(sink),
		rttestingv1.WithSubject(subject),
	)
	client.CreateSinkBindingV1OrFail(sinkBinding)

	// wait for all test resources to be ready, it should fail as subject isn't found
	err := client.WaitForAllTestResourcesReady(ctx)
	if err == nil {
		t.Fatal("sink-binding is ready, but shouldn't be without subject being found")
	}
}

type sinkBindingIsntReadyWithNonExistingSubjectCase struct {
	name       string
	subject    tracker.Reference
	skippedMsg string
}

func sinkBindingIsntReadyWithNonExistingSubjectCases() []sinkBindingIsntReadyWithNonExistingSubjectCase {
	return []sinkBindingIsntReadyWithNonExistingSubjectCase{{
		name: "k8s-deployment-by-name",
		subject: tracker.Reference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "k8s-deployment",
		}}, {
		name: "k8s-service-by-name",
		subject: tracker.Reference{
			APIVersion: "apps/v1",
			Kind:       "Service",
			Name:       "k8s-service",
		}}, {
		// FIXME: #5510 sink binding reports ready even if subject isn't found
		skippedMsg: "FIXME: #5510 sink binding reports ready even if subject isn't found",
		name:       "k8s-deployment-by-match-labels",
		subject: tracker.Reference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app-name": "k8s-deployment-app",
				},
			},
		}}, {
		name: "k8s-service-by-match-labels",
		subject: tracker.Reference{
			APIVersion: "apps/v1",
			Kind:       "Service",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app-name": "k8s-service-app",
				},
			},
		}}, {
		name: "kn-service-by-name",
		subject: tracker.Reference{
			APIVersion: "serving.knative.dev/v1",
			Kind:       "Service",
			Name:       "kn-service",
		}}, {
		name: "kn-service-by-match-labels",
		subject: tracker.Reference{
			APIVersion: "serving.knative.dev/v1",
			Kind:       "Service",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app-name": "kn-service-app",
				},
			},
		}}}
}
