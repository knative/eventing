/*
Copyright 2018 The Knative Authors

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

package resources

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/eventing/pkg/sources"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
)

const (
	feedName      = "feed-name"
	feedNamespace = "feed-namespace"
	feedUid       = "feed-uid"
	feedTarget    = "feed-target"
)

func TestIsJobComplete(t *testing.T) {
	testCases := []struct {
		name       string
		conditions []batchv1.JobCondition
		want       bool
	}{
		{
			name: "no conditions: false",
			want: false,
		},
		{
			name: "no job complete status: false",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
			want: false,
		},
		{
			name: "job complete false: false",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionFalse,
				},
			},
			want: false,
		},
		{
			name: "job complete true: true",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
			want: true,
		},
		{
			name: "multiple conditions, job complete true: true",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := makeJobWithConditions(tc.conditions)
			actual := IsJobComplete(job)
			if actual != tc.want {
				t.Errorf("Expected %v, actual %v", tc.want, actual)
			}
		})
	}
}

func TestIsJobFailed(t *testing.T) {
	testCases := []struct {
		name       string
		conditions []batchv1.JobCondition
		want       bool
	}{
		{
			name: "no conditions: false",
			want: false,
		},
		{
			name: "no job failed status: false",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
			want: false,
		},
		{
			name: "job failed false: false",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionFalse,
				},
			},
			want: false,
		},
		{
			name: "job failed true: true",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
			want: true,
		},
		{
			name: "multiple conditions, job failed true: true",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := makeJobWithConditions(tc.conditions)
			actual := IsJobFailed(job)
			if actual != tc.want {
				t.Errorf("Expected %v, actual %v", tc.want, actual)
			}
		})
	}
}

func TestJobFailedMessage(t *testing.T) {
	jobFailedReason := "test induced job failed reason"
	jobFailedMessage := "test induced job failed message"

	testCases := []struct {
		name       string
		conditions []batchv1.JobCondition
		want       string
	}{
		{
			name: "no conditions: empty string",
			want: "",
		},
		{
			name: "no job failed status: empty string",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
			want: "",
		},
		{
			name: "job failed false: empty string",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionFalse,
				},
			},
			want: "",
		},
		{
			name: "job failed true: message",
			conditions: []batchv1.JobCondition{
				{
					Type:    batchv1.JobFailed,
					Status:  corev1.ConditionTrue,
					Reason:  jobFailedReason,
					Message: jobFailedMessage,
				},
			},
			want: fmt.Sprintf("[%s] %s", jobFailedReason, jobFailedMessage),
		},
		{
			name: "multiple conditions, job failed true: message",
			conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionFalse,
				},
				{
					Type:    batchv1.JobFailed,
					Status:  corev1.ConditionTrue,
					Reason:  jobFailedReason,
					Message: jobFailedMessage,
				},
			},
			want: fmt.Sprintf("[%s] %s", jobFailedReason, jobFailedMessage),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := makeJobWithConditions(tc.conditions)
			actual := JobFailedMessage(job)
			if actual != tc.want {
				t.Errorf("Expected '%v', actual '%v'", tc.want, actual)
			}
		})
	}
}

func TestGetFirstTerminationMessage(t *testing.T) {
	message := "test induced terminal status message"
	testCases := []struct {
		name   string
		status []corev1.ContainerStatus
		want   string
	}{
		{
			name: "no status: empty string",
			want: "",
		},
		{
			name: "no terminated status: empty string",
			status: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.Now(),
						},
					},
				},
			},
			want: "",
		},
		{
			name: "no terminated status message: empty string",
			status: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				},
			},
			want: "",
		},
		{
			name: "terminated status message: message",
			status: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: message,
						},
					},
				},
			},
			want: message,
		},
		{
			name: "multiple terminated status message: first message",
			status: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: message,
						},
					},
				},
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: "some other message that won't be used",
						},
					},
				},
			},
			want: message,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := makePodWithStatus(tc.status)
			actual := GetFirstTerminationMessage(pod)
			if actual != tc.want {
				t.Errorf("Expected '%v', actual '%v'", tc.want, actual)
			}
		})
	}
}

func TestMakeJob(t *testing.T) {
	type jobVerification func(t *testing.T, job *batchv1.Job)

	testCases := []struct {
		name            string
		feed            *feedsv1alpha1.Feed
		source          *feedsv1alpha1.EventSource
		trigger         sources.EventTrigger
		target          string
		wantErr         error
		jobVerification []jobVerification
	}{
		{
			name: "",
			feed: &feedsv1alpha1.Feed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      feedName,
					Namespace: feedNamespace,
					UID:       feedUid,
				},
				Spec: feedsv1alpha1.FeedSpec{
					ServiceAccountName: "feed-service-account",
				},
			},
			source: &feedsv1alpha1.EventSource{},
			trigger: sources.EventTrigger{
				EventType: "test-event-type",
			},
			target: feedTarget,
			jobVerification: []jobVerification{
				jobObjectMetaVerification,
				jobObjectSpecVerification,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, actualErr := MakeJob(tc.feed, tc.source, tc.trigger, tc.target)
			if cmp.Diff(tc.wantErr, actualErr) != "" {
				t.Errorf("Incorrect error (-want, +got): %v", cmp.Diff(tc.wantErr, actualErr))
			}
			for _, jv := range tc.jobVerification {
				jv(t, actual)
			}
		})
	}
}

func makeJobWithConditions(conditions []batchv1.JobCondition) *batchv1.Job {
	return &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: conditions,
		},
	}
}

func makePodWithStatus(status []corev1.ContainerStatus) *corev1.Pod {
	return &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: status,
		},
	}
}

func jobObjectMetaVerification(t *testing.T, job *batchv1.Job) {
	if job.Namespace != feedNamespace {
		t.Errorf("Expected namespace '%v', actual '%v'", feedNamespace, job.Namespace)
	}
	if !strings.Contains(job.Name, feedName) {
		t.Errorf("Expected job name to include the Feed's name, '%v'. Actual: '%v'", feedName, job.Name)
	}
	if job.Labels["app"] != "feedpod" {
		t.Errorf("Missing app=feedpod label. Actual: %v", job.Labels)
	}
	// TODO: Verify the entire owner reference.
	if len(job.OwnerReferences) != 1 || job.OwnerReferences[0].UID != feedUid {
		t.Errorf("Expected exactly one owner reference that points to the Feed. Actual: %+v", job.OwnerReferences)
	}
}

func jobObjectSpecVerification(t *testing.T, job *batchv1.Job) {
	pt := job.Spec.Template
	if pt.Annotations[sidecarIstioInjectAnnotation] != "false" {
		t.Errorf("Expected Istio sidecar injection disabled. Actually: %v", pt.Labels[sidecarIstioInjectAnnotation])
	}
	if len(pt.Spec.Containers) != 1 {
		t.Errorf("Expected exactly one container, the feedlet. Actually: %+v", pt.Spec.Containers)
	}
	c := pt.Spec.Containers[0]
	if getEnvVar(c.Env, "FEED_TARGET") != feedTarget {
		t.Errorf("Expected FEED_TARGET to be '%v'. Actually: %+v", feedTarget, getEnvVar(c.Env, "FEED_TARGET"))
	}
	// TODO: Verify the other fields.
}

func getEnvVar(env []corev1.EnvVar, name string) string {
	for _, ev := range env {
		if ev.Name == name {
			return ev.Value
		}
	}
	return ""
}
