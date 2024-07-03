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
	clientgotesting "k8s.io/client-go/testing"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	jobsinkreconciler "knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/jobsink"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/eventing/pkg/reconciler/testing/v1alpha1"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
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
)

func TestReconcile(t *testing.T) {
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
				NewJobSink(testNamespace, jobSinkName,
					WithJobSinkJob(testJob()),
					WithInitJobSinkConditions),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewJobSink(testNamespace, jobSinkName,
					WithInitJobSinkConditions,
					WithJobSinkJob(testJob()),
					WithJobSinkEventPoliciesReadyBecauseOIDCDisabled()),
			}},
		}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = v1a1addr.WithDuck(ctx)
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

func testJob() *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
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
