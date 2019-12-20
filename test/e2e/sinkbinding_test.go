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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/pkg/tracker"
)

func TestSinkBindingDeployment(t *testing.T) {
	const (
		sinkBindingName = "e2e-sink-binding"
		deploymentName  = "e2e-sink-binding-deployment"
		// the heartbeats image is built from test_images/heartbeats
		imageName = "heartbeats"

		loggerPodName = "e2e-sink-binding-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

	// create sink binding
	sinkBinding := eventingtesting.NewSinkBinding(
		sinkBindingName,
		client.Namespace,
		eventingtesting.WithSink(duckv1.Destination{Ref: resources.ServiceRef(loggerPodName)}),
		eventingtesting.WithSubject(tracker.Reference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  client.Namespace,
			Name:       deploymentName,
		}),
	)
	client.CreateLegacySinkBindingOrFail(sinkBinding)

	data := fmt.Sprintf("TestSinkBindingDeployment%s", uuid.NewUUID())
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
						ImagePullPolicy: corev1.PullAlways,
						Args:            []string{"--msg=" + data},
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
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// verify the logger service receives the event
	expectedCount := 2
	if err := client.CheckLog(loggerPodName, common.CheckerContainsAtLeast(data, expectedCount)); err != nil {
		t.Fatalf("String %q does not appear at least %d times in logs of logger pod %q: %v", data, expectedCount, loggerPodName, err)
	}
}

func TestSinkBindingCronJob(t *testing.T) {
	const (
		sinkBindingName = "e2e-sink-binding"
		deploymentName  = "e2e-sink-binding-cronjob"
		// the heartbeats image is built from test_images/heartbeats
		imageName = "heartbeats"

		loggerPodName = "e2e-sink-binding-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

	// create sink binding
	sinkBinding := eventingtesting.NewSinkBinding(
		sinkBindingName,
		client.Namespace,
		eventingtesting.WithSink(duckv1.Destination{Ref: resources.ServiceRef(loggerPodName)}),
		eventingtesting.WithSubject(tracker.Reference{
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
	client.CreateLegacySinkBindingOrFail(sinkBinding)

	data := fmt.Sprintf("TestSinkBindingCronJob%s", uuid.NewUUID())
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
								ImagePullPolicy: corev1.PullAlways,
								Args:            []string{"--msg=" + data},
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
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// verify the logger service receives the event
	expectedCount := 2
	if err := client.CheckLog(loggerPodName, common.CheckerContainsAtLeast(data, expectedCount)); err != nil {
		t.Fatalf("String %q does not appear at least %d times in logs of logger pod %q: %v", data, expectedCount, loggerPodName, err)
	}
}
