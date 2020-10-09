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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	. "github.com/cloudevents/sdk-go/v2/test"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"

	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestSinkBindingV1Beta1Deployment(t *testing.T) {
	const (
		sinkBindingName = "e2e-sink-binding"
		deploymentName  = "e2e-sink-binding-deployment"
		// the heartbeats image is built from test_images/heartbeats
		imageName = "heartbeats"

		recordEventPodName = "e2e-sink-binding-recordevent-pod-v1beta1dep"
	)

	client := setup(t, true)
	defer tearDown(client)

	ctx := context.Background()

	// create event logger pod and service
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventPodName)
	extensionSecret := string(uuid.NewUUID())

	// create sink binding
	sinkBinding := eventingtesting.NewSinkBindingV1Beta1(
		sinkBindingName,
		client.Namespace,
		eventingtesting.WithSinkV1B1(duckv1.Destination{Ref: resources.KnativeRefForService(recordEventPodName, client.Namespace)}),
		eventingtesting.WithSubjectV1B1(tracker.Reference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  client.Namespace,
			Name:       deploymentName,
		}),
		eventingtesting.WithCloudEventOverridesV1B1(duckv1.CloudEventOverrides{Extensions: map[string]string{
			"sinkbinding": extensionSecret,
		}}),
	)
	client.CreateSinkBindingV1Beta1OrFail(sinkBinding)

	message := fmt.Sprintf("TestSinkBindingDeployment%s", uuid.NewUUID())
	client.CreateDeploymentOrFail(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: client.Namespace,
			Name:      deploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            imageName,
						Image:           pkgTest.ImagePath(imageName),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args:            []string{"--msg=" + message},
						Env: []corev1.EnvVar{{
							Name:  "POD_NAME",
							Value: deploymentName,
						}, {
							Name:  "POD_NAMESPACE",
							Value: client.Namespace,
						}},
					}},
				},
			},
		},
	})

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// Look for events with expected data, and sinkbinding extension
	eventTracker.AssertAtLeast(2, recordevents.MatchEvent(
		recordevents.MatchHeartBeatsImageMessage(message),
		HasSource(fmt.Sprintf("https://knative.dev/eventing/test/heartbeats/#%s/%s", client.Namespace, deploymentName)),
		HasExtension("sinkbinding", extensionSecret),
	))
}

func TestSinkBindingV1Beta1CronJob(t *testing.T) {
	const (
		sinkBindingName = "e2e-sink-binding"
		deploymentName  = "e2e-sink-binding-cronjob"
		// the heartbeats image is built from test_images/heartbeats
		imageName = "heartbeats"

		recordEventPodName = "e2e-sink-binding-recordevent-pod-v1beta1c"
	)

	client := setup(t, true)
	defer tearDown(client)

	ctx := context.Background()

	// create event logger pod and service
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventPodName)
	// create sink binding
	sinkBinding := eventingtesting.NewSinkBindingV1Beta1(
		sinkBindingName,
		client.Namespace,
		eventingtesting.WithSinkV1B1(duckv1.Destination{Ref: resources.KnativeRefForService(recordEventPodName, client.Namespace)}),
		eventingtesting.WithSubjectV1B1(tracker.Reference{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Namespace:  client.Namespace,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		}),
	)
	client.CreateSinkBindingV1Beta1OrFail(sinkBinding)

	message := fmt.Sprintf("TestSinkBindingCronJob%s", uuid.NewUUID())
	client.CreateCronJobOrFail(&batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: client.Namespace,
			Name:      deploymentName,
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: "* * * * *",
			JobTemplate: batchv1beta1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{{
								Name:            imageName,
								Image:           pkgTest.ImagePath(imageName),
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args:            []string{"--msg=" + message},
								Env: []corev1.EnvVar{{
									Name:  "ONE_SHOT",
									Value: "true",
								}, {
									Name:  "POD_NAME",
									Value: deploymentName,
								}, {
									Name:  "POD_NAMESPACE",
									Value: client.Namespace,
								}},
							}},
						},
					},
				},
			},
		},
	})

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// verify the logger service receives the event
	eventTracker.AssertAtLeast(2, recordevents.MatchEvent(
		recordevents.MatchHeartBeatsImageMessage(message),
		HasSource(fmt.Sprintf("https://knative.dev/eventing/test/heartbeats/#%s/%s", client.Namespace, deploymentName)),
	))

}
