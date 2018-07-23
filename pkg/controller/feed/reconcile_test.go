/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package feed

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/eventing/pkg/controller/feed/resources"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/sources"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
)

/*
TODO
- initial: feed with job deadline exceeded
  reconciled: feed failure, job exists, finalizer
*/

var (
	trueVal  = true
	falseVal = false
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()
)

func init() {
	// Add types to scheme
	feedsv1alpha1.AddToScheme(scheme.Scheme)
	servingv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new feed: adds status, finalizer, creates job",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getNewFeed(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getStartInProgressFeed(),
			getNewStartJob(),
			//TODO job created event
		},
	},
	{
		Name: "in progress feed with existing job: both unchanged",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getStartInProgressFeed(),
			getNewStartJob(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getStartInProgressFeed(),
			getNewStartJob(),
		},
	},
	{
		Name: "in progress feed with completed job: updated status, context, job exists",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getStartInProgressFeed(),
			getCompletedStartFeedJob(),
			getCompletedStartFeedJobPod(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getStartedFeed(),
			getCompletedStartFeedJob(),
			//TODO job completed event
		},
	},
	{
		Name: "in progress feed with failed start job: updated status, job exists",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getStartInProgressFeed(),
			getFailedStartFeedJob(),
			getCompletedStartFeedJobPod(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getStartFailedFeed(),
			getFailedStartFeedJob(),
			//TODO job failed event
		},
	},
	{
		Name: "Deleted feed with finalizer, previously completed, feed job exists: feed job deleted",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getDeletedStartedFeed(),
			getCompletedStartFeedJob(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getDeletedStartedFeed(),
		},
		WantAbsent: []runtime.Object{
			getCompletedStartFeedJob(),
		},
	},
	{
		Name: "Deleted feed with finalizer, previously completed, feed job missing: stop feed job created, status updated",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getDeletedStartedFeed(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getDeletedStopInProgressFeed(),
			getNewStopJob(),
			//TODO job created event
		},
	},
	{
		Name: "Deleted in-progress feed with finalizer, stop feed job exists: unchanged",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getDeletedStopInProgressFeed(),
			getInProgressStopJob(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getDeletedStopInProgressFeed(),
			getInProgressStopJob(),
		},
	},
	{
		Name: "Deleted feed with completed stop feed job: no finalizers, update status",
		InitialState: []runtime.Object{
			getEventSource(),
			getEventType(),
			getRoute(),
			getDeletedStopInProgressFeed(),
			getCompletedStopJob(),
		},
		ReconcileKey: "test/test-feed",
		WantPresent: []runtime.Object{
			getDeletedStoppedFeed(),
			//TODO job completed event
		},
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		r := &reconciler{
			client:   tc.GetClient(),
			recorder: recorder,
		}
		t.Run(tc.Name, tc.Runner(t, r, r.client))
	}
}

func getEventSource() *feedsv1alpha1.EventSource {
	return &feedsv1alpha1.EventSource{
		ObjectMeta: om("test", "test-es"),
		Spec: feedsv1alpha1.EventSourceSpec{
			CommonEventSourceSpec: feedsv1alpha1.CommonEventSourceSpec{
				Source:     "github",
				Image:      "example.com/test-es-feeder",
				Parameters: nil,
			},
		},
	}
}

func getEventType() *feedsv1alpha1.EventType {
	return &feedsv1alpha1.EventType{
		ObjectMeta: om("test", "test-et"),
		Spec: feedsv1alpha1.EventTypeSpec{
			EventSource: getEventSource().Name,
		},
	}
}

func getRoute() *servingv1alpha1.Route {
	return &servingv1alpha1.Route{
		ObjectMeta: om("test", "test-route"),
		Spec:       servingv1alpha1.RouteSpec{},
		Status: servingv1alpha1.RouteStatus{
			Domain: "example.com",
		},
	}
}

func getFeedContext() *sources.FeedContext {
	return &sources.FeedContext{
		Context: map[string]interface{}{
			"foo": "bar",
		},
	}
}

func getNewFeed() *feedsv1alpha1.Feed {
	return &feedsv1alpha1.Feed{
		TypeMeta:   feedType(),
		ObjectMeta: om("test", "test-feed"),
		Spec: feedsv1alpha1.FeedSpec{
			Action: feedsv1alpha1.FeedAction{
				RouteName: getRoute().Name,
			},
			Trigger: feedsv1alpha1.EventTrigger{
				EventType:      getEventType().Name,
				Resource:       "",
				Service:        "",
				Parameters:     nil,
				ParametersFrom: nil,
			},
		},
	}
}

func getStartInProgressFeed() *feedsv1alpha1.Feed {
	feed := getNewFeed()
	feed.SetOwnerReference(&metav1.OwnerReference{
		APIVersion:         feedsv1alpha1.SchemeGroupVersion.String(),
		Kind:               "EventType",
		Name:               getEventType().Name,
		Controller:         &falseVal,
		BlockOwnerDeletion: &trueVal,
	})
	feed.AddFinalizer(finalizerName)

	feed.Status.InitializeConditions()
	feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
		Type:    feedsv1alpha1.FeedConditionReady,
		Status:  corev1.ConditionUnknown,
		Reason:  "StartJob",
		Message: "start job in progress",
	})

	return feed
}

func getStartedFeed() *feedsv1alpha1.Feed {
	feed := getStartInProgressFeed()
	marshalledContext, err := json.Marshal(getFeedContext().Context)
	if err != nil {
		panic(err)
	}
	feed.Status.FeedContext = &runtime.RawExtension{
		Raw: marshalledContext,
	}
	feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
		Type:    feedsv1alpha1.FeedConditionReady,
		Status:  corev1.ConditionTrue,
		Reason:  "FeedSuccess",
		Message: "start job succeeded",
	})
	return feed
}

func getStartFailedFeed() *feedsv1alpha1.Feed {
	feed := getStartInProgressFeed()
	feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
		Type:    feedsv1alpha1.FeedConditionReady,
		Status:  corev1.ConditionFalse,
		Reason:  "FeedFailed",
		Message: "Job failed with [] ",
	})
	return feed
}

func getDeletedStartedFeed() *feedsv1alpha1.Feed {
	feed := getStartedFeed()
	feed.SetDeletionTimestamp(&deletionTime)
	return feed
}

func getDeletedStopInProgressFeed() *feedsv1alpha1.Feed {
	feed := getDeletedStartedFeed()

	feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
		Type:    feedsv1alpha1.FeedConditionReady,
		Status:  corev1.ConditionUnknown,
		Reason:  "StopJob",
		Message: "stop job in progress",
	})
	return feed
}

func getDeletedStoppedFeed() *feedsv1alpha1.Feed {
	feed := getDeletedStopInProgressFeed()
	feed.RemoveFinalizer(finalizerName)
	feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
		Type:    feedsv1alpha1.FeedConditionReady,
		Status:  corev1.ConditionTrue,
		Reason:  "FeedSuccess",
		Message: "stop job succeeded",
	})
	return feed
}

func getNewStartJob() *batchv1.Job {
	jobName := resources.StartJobName(getNewFeed())
	return &batchv1.Job{
		TypeMeta: jobType(),
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      jobName,
			Labels:    map[string]string{"app": "feedpod"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         feedsv1alpha1.SchemeGroupVersion.String(),
				Kind:               "Feed",
				Name:               getNewFeed().Name,
				Controller:         &trueVal,
				BlockOwnerDeletion: &trueVal,
			}},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"sidecar.istio.io/inject": "false"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "feedlet",
						Image: "example.com/test-es-feeder",
						Env: []corev1.EnvVar{{
							Name:  string(resources.EnvVarOperation),
							Value: string(resources.OperationStartFeed),
						}, {
							Name:  string(resources.EnvVarTarget),
							Value: "example.com",
						}, {
							Name: string(resources.EnvVarTrigger),
							Value: base64.StdEncoding.EncodeToString(bytesOrDie(json.Marshal(
								sources.EventTrigger{
									EventType:  "test-et",
									Parameters: map[string]interface{}{},
								},
							))),
						}, {
							Name: string(resources.EnvVarContext),
							Value: base64.StdEncoding.EncodeToString(bytesOrDie(json.Marshal(
								sources.FeedContext{},
							))),
						}, {
							Name:  string(resources.EnvVarEventSourceParameters),
							Value: "",
						}, {
							Name: string(resources.EnvVarNamespace),
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						}, {
							Name: string(resources.EnvVarServiceAccount),
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "spec.serviceAccountName",
								},
							},
						}},
						ImagePullPolicy: corev1.PullAlways,
					}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			BackoffLimit:          &resources.DefaultBackoffLimit,
			ActiveDeadlineSeconds: &resources.DefaultActiveDeadlineSeconds,
		},
		Status: batchv1.JobStatus{},
	}
}

func getInProgressStartFeedJob() *batchv1.Job {
	job := getNewStartJob()
	// This is normally set by a webhook. Set it here
	// to simulate. TODO use a reactor when that's
	// supported.
	job.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"job": job.Name,
		},
	}
	return job
}

func getCompletedStartFeedJob() *batchv1.Job {
	job := getInProgressStartFeedJob()
	job.Status = batchv1.JobStatus{
		Conditions: []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}},
	}
	return job
}

func getFailedStartFeedJob() *batchv1.Job {
	job := getInProgressStartFeedJob()
	job.Status = batchv1.JobStatus{
		Conditions: []batchv1.JobCondition{{
			Type:   batchv1.JobFailed,
			Status: corev1.ConditionTrue,
		}},
	}
	return job
}

func getCompletedStartFeedJobPod() *corev1.Pod {
	job := getCompletedStartFeedJob()
	outputContext, err := json.Marshal(getFeedContext())
	if err != nil {
		panic(err)
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      job.Name,
			Labels:    map[string]string{"job": job.Name},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: base64.StdEncoding.EncodeToString(outputContext),
					},
				},
			}},
		},
	}
}

func getNewStopJob() *batchv1.Job {
	job := getNewStartJob()
	job.Name = resources.StopJobName(getDeletedStartedFeed())

	job.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{
		Name:  string(resources.EnvVarOperation),
		Value: string(resources.OperationStopFeed),
	}, {
		Name:  string(resources.EnvVarTarget),
		Value: "example.com",
	}, {
		Name: string(resources.EnvVarTrigger),
		Value: base64.StdEncoding.EncodeToString(bytesOrDie(json.Marshal(
			sources.EventTrigger{
				EventType:  "test-et",
				Parameters: map[string]interface{}{},
			},
		))),
	}, {
		Name:  string(resources.EnvVarContext),
		Value: base64.StdEncoding.EncodeToString(bytesOrDie(json.Marshal(getFeedContext()))),
	}, {
		Name:  string(resources.EnvVarEventSourceParameters),
		Value: "",
	}, {
		Name: string(resources.EnvVarNamespace),
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, {
		Name: string(resources.EnvVarServiceAccount),
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "spec.serviceAccountName",
			},
		},
	}}
	return job
}

func getInProgressStopJob() *batchv1.Job {
	job := getNewStopJob()
	// This is normally set by a webhook. Set it here
	// to simulate. TODO use a reactor when that's
	// supported.
	job.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"job": job.Name,
		},
	}
	return job
}

func getCompletedStopJob() *batchv1.Job {
	job := getInProgressStopJob()
	job.Status = batchv1.JobStatus{
		Conditions: []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}},
	}
	return job
}

func getFailedStopJob() *batchv1.Job {
	job := getInProgressStopJob()
	job.Status = batchv1.JobStatus{
		Conditions: []batchv1.JobCondition{{
			Type:   batchv1.JobFailed,
			Status: corev1.ConditionTrue,
		}},
	}
	return job
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func bytesOrDie(v []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return v
}

func feedType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: feedsv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Feed",
	}
}

func jobType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: batchv1.SchemeGroupVersion.String(),
		Kind:       "Job",
	}
}

func podType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Pod",
	}
}
