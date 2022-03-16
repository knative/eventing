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

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

// WaitUntilJobDone waits until a job has finished.
// Timing is optional but if provided is [interval, timeout].
func WaitUntilJobDone(ctx context.Context, t feature.T, name string, timing ...time.Duration) error {
	interval, timeout := PollTimings(ctx, timing)
	namespace := environment.FromContext(ctx).Namespace()
	kube := kubeclient.Get(ctx)
	jobs := kube.BatchV1().Jobs(namespace)

	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		job, err := jobs.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("%s/%s job %+v", namespace, name, err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		complete := IsJobComplete(job)
		if !complete {
			status, err := json.Marshal(job.Status)
			if err != nil {
				return false, err
			}
			t.Logf("%s/%s job status %s", namespace, name, status)
		}
		return complete, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// WaitForJobTerminationMessage waits for a job to end and then collects the termination message.
// Timing is optional but if provided is [interval, timeout].
func WaitForJobTerminationMessage(ctx context.Context, t feature.T, name string, timing ...time.Duration) (string, error) {
	interval, timeout := PollTimings(ctx, timing)
	namespace := environment.FromContext(ctx).Namespace()

	// poll until the pod is terminated.
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pod, err := GetJobPodByJobName(ctx, name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("%s/%s job termination %+v", namespace, name, err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		if pod != nil {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Terminated != nil {
					return true, nil
				}
			}
		}
		return false, nil
	})

	if err != nil {
		return "", err
	}
	pod, err := GetJobPodByJobName(ctx, name)
	if err != nil {
		return "", err
	}
	return GetFirstTerminationMessage(pod), nil
}

func IsJobComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsJobSucceeded(job *batchv1.Job) bool {
	return !IsJobFailed(job)
}

func IsJobFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func JobFailedMessage(job *batchv1.Job) string {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return fmt.Sprintf("[%s] %s", c.Reason, c.Message)
		}
	}
	return ""
}

// GetJobPod will find the Pod that belongs to the resource that created it.
// Uses label ""controller-uid  as the label selector. So, your job should
// tag the job with that label as the UID of the resource that's needing it.
// For example, if you create a storage object that requires us to create
// a notification for it, the controller should set the label on the
// Job responsible for creating the Notification for it with the label
// "controller-uid" set to the uid of the storage CR.
func GetJobPod(ctx context.Context, kubeClientset kubernetes.Interface, namespace, uid, operation string) (*corev1.Pod, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("Looking for Pod with UID: %q action: %q", uid, operation)
	matchLabels := map[string]string{
		"resource-uid": uid,
		"action":       operation,
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		logger.Infof("Found pod: %q", pod.Name)
		return &pod, nil
	}
	return nil, fmt.Errorf("Pod not found")
}

// GetJobPodByJobName will find the Pods that belong to that job. Each pod
// for a given job will have label called: "job-name" set to the job that
// it belongs to, so just filter by that.
func GetJobPodByJobName(ctx context.Context, jobName string) (*corev1.Pod, error) {
	logger := logging.FromContext(ctx)
	namespace := environment.FromContext(ctx).Namespace()
	kube := kubeclient.Get(ctx)
	logger.Infof("Looking for Pod with jobname: %q", jobName)
	matchLabels := map[string]string{
		"job-name": jobName,
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}
	pods, err := kube.CoreV1().Pods(namespace).
		List(ctx, metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		logger.Infof("Found pod: %q", pod.Name)
		return &pod, nil
	}
	return nil, fmt.Errorf("Pod not found")
}
