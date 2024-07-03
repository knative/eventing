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

package jobsink

import (
	"context"
	"fmt"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	jobsinkreconciler "knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/jobsink"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/eventing/pkg/reconciler/testing/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1 "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	// TestNamespace is the namespace used for testing.
	testNamespace = "test-namespace"
	// TestName is the name used for testing.
	jobSinkName = "test-jobSink"

	readyEventPolicyName   = "ready-event-policy"
	unreadyEventPolicyName = "unready-event-policy"
)

var (
	testKey = fmt.Sprintf("%s/%s", testNamespace, jobSinkName)

	jobSinkAddressable = duckv1.Addressable{
		Name: pointer.String("http"),
		URL: &apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname("job-sink", testNamespace),
			Path:   fmt.Sprintf("/%s/%s", testNamespace, jobSinkName),
		},
	}

	jobSinkGVK = metav1.GroupVersionKind{
		Group:   "sinks.knative.dev",
		Version: "v1alpha1",
		Kind:    "JobSink",
	}
)

func TestReconcile(t *testing.T) {
	// Seed the random number generator for deterministic results
	seed := int64(42)
	utilrand.Seed(seed)

	table := TableTest{
		{
			Name: "bad work queue key",
			Key:  "too/many/parts",
		},
		{
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "JobSink not found",
			Key:  testKey,
		}, {
			Name: "Successful reconciliation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewJobSink(jobSinkName, testNamespace,
					WithJobSinkJob(testJob("")),
					WithInitJobSinkConditions),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				testJob("test-jobSinkzdfkp"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewJobSink(jobSinkName, testNamespace,
						WithJobSinkJob(testJob("")),
						WithJobSinkAddressableReady(),
						WithJobSinkJobStatusSelector(),
						WithJobSinkAddress(&jobSinkAddressable),
						WithJobSinkEventPoliciesReadyBecauseOIDCDisabled()),
				},
			},
		}, {
			Name: "Should provision applying EventPolicies",
			Key:  testKey,
			Objects: []runtime.Object{
				NewJobSink(jobSinkName, testNamespace,
					WithJobSinkJob(testJob("")),
					WithInitJobSinkConditions),
				NewEventPolicy(readyEventPolicyName, testNamespace,
					WithReadyEventPolicyCondition,
					WithEventPolicyToRef(jobSinkGVK, jobSinkName),
				),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				testJob("test-jobSink7h6ts"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewJobSink(jobSinkName, testNamespace,
						WithJobSinkJob(testJob("")),
						WithJobSinkAddressableReady(),
						WithJobSinkJobStatusSelector(),
						WithJobSinkAddress(&jobSinkAddressable),
						WithJobSinkEventPoliciesReady(),
						WithJobSinkEventPoliciesListed(readyEventPolicyName),
					),
				},
			},
		}, {
			Name: "Should mark as NotReady on unready EventPolicies",
			Key:  testKey,
			Objects: []runtime.Object{
				NewJobSink(jobSinkName, testNamespace,
					WithJobSinkJob(testJob("")),
					WithInitJobSinkConditions),
				NewEventPolicy(unreadyEventPolicyName, testNamespace,
					WithUnreadyEventPolicyCondition("", ""),
					WithEventPolicyToRef(jobSinkGVK, jobSinkName),
				),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				testJob("test-jobSinklk6hn"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewJobSink(jobSinkName, testNamespace,
						WithJobSinkJob(testJob("")),
						WithJobSinkAddressableReady(),
						WithJobSinkJobStatusSelector(),
						WithJobSinkAddress(&jobSinkAddressable),
						WithJobSinkEventPoliciesNotReady("EventPoliciesNotReady", fmt.Sprintf("event policies %s are not ready", unreadyEventPolicyName)),
					),
				},
			},
		}, {
			Name: "Should list only ready EventPolicies",
			Key:  testKey,
			Objects: []runtime.Object{
				NewJobSink(jobSinkName, testNamespace,
					WithJobSinkJob(testJob("")),
					WithInitJobSinkConditions),
				NewEventPolicy(unreadyEventPolicyName, testNamespace,
					WithUnreadyEventPolicyCondition("", ""),
					WithEventPolicyToRef(jobSinkGVK, jobSinkName),
				),
				NewEventPolicy(readyEventPolicyName, testNamespace,
					WithReadyEventPolicyCondition,
					WithEventPolicyToRef(jobSinkGVK, jobSinkName),
				),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				testJob("test-jobSinkmlxzr"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewJobSink(jobSinkName, testNamespace,
						WithJobSinkJob(testJob("")),
						WithJobSinkAddressableReady(),
						WithJobSinkJobStatusSelector(),
						WithJobSinkAddress(&jobSinkAddressable),
						WithJobSinkEventPoliciesNotReady("EventPoliciesNotReady", fmt.Sprintf("event policies %s are not ready", unreadyEventPolicyName)),
						WithJobSinkEventPoliciesListed(readyEventPolicyName),
					),
				},
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = v1.WithDuck(ctx)
		r := &Reconciler{
			jobLister:         listers.GetJobLister(),
			secretLister:      listers.GetSecretLister(),
			eventPolicyLister: listers.GetEventPolicyLister(),
			systemNamespace:   testNamespace,
		}

		return jobsinkreconciler.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetJobSinkLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
	))
}

func testJob(name string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
		},
	}
}
