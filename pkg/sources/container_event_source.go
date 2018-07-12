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

package sources

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	v1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ContainerEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	// Namespace to run the container in
	Namespace string

	// ServiceAccount to run as
	ServiceAccountName string

	// Feed for this operation
	Feed *v1alpha1.Feed

	// EventSourceSpec for the actual underlying source
	EventSourceSpec *v1alpha1.EventSourceSpec
}

func NewContainerEventSource(feed *v1alpha1.Feed, kubeclientset kubernetes.Interface, spec *v1alpha1.EventSourceSpec, namespace string, serviceAccountName string) EventSource {
	return &ContainerEventSource{
		kubeclientset:      kubeclientset,
		Namespace:          namespace,
		ServiceAccountName: serviceAccountName,
		Feed:               feed,
		EventSourceSpec:    spec,
	}
}

func (t *ContainerEventSource) StartFeed(trigger EventTrigger, route string) (*FeedContext, error) {
	job, err := MakeJob(t.Feed, t.Namespace, t.ServiceAccountName, "feed-starter", t.EventSourceSpec, StartFeed, trigger, route, FeedContext{})
	if err != nil {
		glog.Errorf("failed to make job: %s", err)
		return nil, err
	}
	bc, err := t.run(job, true)
	if err != nil {
		glog.Errorf("failed to run feed start job: %s", err)
	}
	// Try to delete the job even if it failed to run
	delErr := t.delete(job)
	if delErr != nil {
		glog.Errorf("failed to delete the job after running: %s", delErr)
	}
	return bc, err
}

func (t *ContainerEventSource) StopFeed(trigger EventTrigger, feedContext FeedContext) error {
	job, err := MakeJob(t.Feed, t.Namespace, t.ServiceAccountName, "feed-stopper", t.EventSourceSpec, StopFeed, trigger, "", feedContext)
	if err != nil {
		glog.Errorf("failed to make job: %s", err)
		return err
	}
	_, err = t.run(job, false)
	if err != nil {
		glog.Errorf("failed to run feed stop job: %s", err)
	}
	// Try to delete the job anyways
	delErr := t.delete(job)
	if delErr != nil {
		glog.Errorf("failed to delete the job after running: %s", delErr)
	}
	return err
}

func (t *ContainerEventSource) run(job *batchv1.Job, parseLogs bool) (*FeedContext, error) {
	jobClient := t.kubeclientset.BatchV1().Jobs(job.Namespace)
	_, err := jobClient.Create(job)
	if err != nil {
		return &FeedContext{}, err
	}

	// TODO replace with a construct similar to Build by watching for pod
	// notifications and use channels for unblocking.
	for {
		time.Sleep(300 * time.Millisecond)
		glog.Infof("Checking job...")
		job, err := jobClient.Get(job.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if isJobFailed(job) {
			glog.Errorf("event source job failed: %s", err)
			return nil, fmt.Errorf("Job failed: %s", err)
		}

		if isJobComplete(job) {
			glog.Infof("event source job complete")

			pods, err := t.getJobPods(job)
			if err != nil {
				glog.Errorf("Failed to get job pods: %s", err)
			}

			for _, p := range pods {
				if p.Status.Phase == corev1.PodSucceeded {
					glog.Infof("Pod succeeded: %s", p.Name)
					if msg := getFirstTerminationMessage(&p); msg != "" {
						decodedContext, _ := base64.StdEncoding.DecodeString(msg)
						glog.Infof("Decoded to %q", decodedContext)
						var ret FeedContext
						err = json.Unmarshal(decodedContext, &ret)
						if err != nil {
							glog.Errorf("Failed to unmarshal context: %s", err)
							return nil, err
						}
						return &ret, nil
					}
				}
			}

			return &FeedContext{}, nil
		}
	}
}

func (t *ContainerEventSource) delete(job *batchv1.Job) error {
	jobClient := t.kubeclientset.BatchV1().Jobs(job.Namespace)
	backgroundPolicy := metav1.DeletePropagationBackground
	err := jobClient.Delete(job.Name, &metav1.DeleteOptions{
		PropagationPolicy: &backgroundPolicy,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Infof("Job has already been deleted")
			return nil
		}
		glog.Errorf("failed to delete job: %s", err)
	}
	return err
}

func (t *ContainerEventSource) getJobPods(job *batchv1.Job) ([]corev1.Pod, error) {
	selector, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return nil, err
	}

	podClient := t.kubeclientset.CoreV1().Pods(job.Namespace)
	pods, err := podClient.List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func isJobFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isJobComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func getFirstTerminationMessage(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.Message != "" {
			return cs.State.Terminated.Message
		}
	}
	return ""
}
