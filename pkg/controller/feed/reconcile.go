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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/eventing/pkg/controller/feed/resources"
	"github.com/knative/eventing/pkg/sources"
	yaml "gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = controllerAgentName
)

// Reconcile compares the actual state of a Feed with the desired, and attempts
// to converge the two. It then updates the Status block of the Feed with
// its current state.
// If Reconcile returns a non-nil error, the request will be retried.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	//TODO use this to store the logger and set a deadline
	ctx := context.TODO()

	feed := &feedsv1alpha1.Feed{}
	err := r.client.Get(ctx, request.NamespacedName, feed)

	// The feed may have been deleted since it was added to the workqueue. If so
	// there's nothing to be done.
	if errors.IsNotFound(err) {
		glog.Errorf("could not find Feed %v\n", request)
		return reconcile.Result{}, nil
	}

	// If the feed exists but could not be retrieved, then we should retry.
	if err != nil {
		glog.Errorf("could not fetch Feed %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	// Now that we know the feed exists, we can reconcile it. An error returned
	// here means the reconcile did not complete and the Feed should be requeued
	// for another attempt.
	// A successful reconcile does not necessarily mean the feed is in the desired
	// state, it means no more can be done for now. In this case the feed will
	// not be reconciled again until the resync period or a watched resource
	// changes.
	if err = r.reconcile(ctx, feed); err != nil {
		glog.Errorf("error reconciling Feed: %v", err)
	}

	// Since the reconcile is a sequence of steps, earlier steps may complete
	// successfully while later steps fail. The Feed is updated on failure to
	// preserve any useful status or metadata changes the non-failing steps made.
	if updateErr := r.updateFeed(ctx, feed); updateErr != nil {
		glog.Errorf("failed to update Feed: %v", updateErr)
		// An error here means the feed should be reconciled again, regardless of
		// whether the reconcile was successful or not.
		return reconcile.Result{}, updateErr
	}
	return reconcile.Result{}, err
}

// reconcile tries to converge the current state of the given Feed to the
// desired state. This function should not update the Feed; the calling method
// should do that.
func (r *reconciler) reconcile(ctx context.Context, feed *feedsv1alpha1.Feed) error {
	feed.Status.InitializeConditions()
	// Fetch the EventSource and EventType that is being asked for
	// and if they don't exist update the status to reflect this back
	// to the user.
	eventSource, eventType, err := r.getFeedSource(ctx, feed)
	if err != nil {
		switch t := err.(type) {
		case *EventSourceError:
			glog.Errorf("eventsource can not be used as a target : %s", err)
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionDependenciesSatisfied,
				Status:  corev1.ConditionFalse,
				Reason:  t.Reason,
				Message: t.Message,
			})
		case *EventTypeError:
			glog.Errorf("eventtype can not be used as a target : %s", err)
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionDependenciesSatisfied,
				Status:  corev1.ConditionFalse,
				Reason:  t.Reason,
				Message: t.Message,
			})
		default:
			// This is unreachable unless getFeedSource is refactored.
			// Assume this is a non-terminal state and return the error.
			return fmt.Errorf("Error getting feed source: %v", err)
		}
		// This is a terminal state, and we've noted it in the status, so return
		// nil to signal that no further reconciling is required.
		return nil
	}

	// TODO: Set the FeedConditionDependenciesSatisfied to true here? Or, after
	// job finishes? For now, the above conveys enough information to the user
	// to make sure if they make a typo they will get that relayed back to them.
	// IF we make it here, clear the condition in case they actually fixed the problem
	// say, by installing an event provider.
	feed.Status.RemoveCondition(feedsv1alpha1.FeedConditionDependenciesSatisfied)

	// Once we found the actual type, set the owner reference for it.
	// TODO: Remove this and use finalizers in the EventTypes / EventSources
	// to do this properly.
	// TODO: Add issue link here. can't look up right now, no wifi
	r.setEventTypeOwnerReference(ctx, feed)

	if feed.GetDeletionTimestamp() == nil {
		err = r.reconcileStartJob(ctx, feed, eventSource, eventType)
		if err != nil {
			glog.Errorf("error reconciling start Job: %v", err)
		}
	} else {
		err = r.reconcileStopJob(ctx, feed, eventSource, eventType)
		if err != nil {
			glog.Errorf("error reconciling stop Job: %v", err)
		}
	}
	return nil
}

// reconcileStartJob creates a start Job if one doesn't exist, checks the status
// of the start Job, and updates the Feed status accordingly.
func (r *reconciler) reconcileStartJob(ctx context.Context, feed *feedsv1alpha1.Feed, es *feedsv1alpha1.EventSource, et *feedsv1alpha1.EventType) error {
	bc := feed.Status.GetCondition(feedsv1alpha1.FeedConditionReady)
	switch bc.Status {
	case corev1.ConditionUnknown:

		job := &batchv1.Job{}
		jobName := resources.JobName(feed)

		if err := r.client.Get(ctx, client.ObjectKey{Namespace: feed.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(ctx, feed, es, et)
				if err != nil {
					return err
				}
				r.recorder.Eventf(feed, corev1.EventTypeNormal, "StartJobCreated", "Created start job %q", job.Name)
				feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
					Type:    feedsv1alpha1.FeedConditionReady,
					Status:  corev1.ConditionUnknown,
					Reason:  "StartJob",
					Message: "start job in progress",
				})
			}
		}
		feed.AddFinalizer(finalizerName)

		if resources.IsJobComplete(job) {
			r.recorder.Eventf(feed, corev1.EventTypeNormal, "StartJobCompleted", "Start job %q completed", job.Name)
			if err := r.setFeedContext(ctx, feed, job); err != nil {
				return err
			}
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionReady,
				Status:  corev1.ConditionTrue,
				Reason:  "FeedSuccess",
				Message: "start job succeeded",
			})
		} else if resources.IsJobFailed(job) {
			r.recorder.Eventf(feed, corev1.EventTypeWarning, "StartJobFailed", "Start job %q failed: %q", job.Name, resources.JobFailedMessage(job))
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionReady,
				Status:  corev1.ConditionFalse,
				Reason:  "FeedFailed",
				Message: fmt.Sprintf("Job failed with %s", resources.JobFailedMessage(job)),
			})
		}

	case corev1.ConditionTrue:
		//TODO delete job
	}
	return nil
}

// reconcileStopJob deletes the start Job if it exists, creates a stop Job if
// one doesn't exist, checks the status of the stop Job, and updates the Feed
// status accordingly.
func (r *reconciler) reconcileStopJob(ctx context.Context, feed *feedsv1alpha1.Feed, es *feedsv1alpha1.EventSource, et *feedsv1alpha1.EventType) error {
	if feed.HasFinalizer(finalizerName) {

		// check for an existing start Job
		job := &batchv1.Job{}
		jobName := resources.StartJobName(feed)

		err := r.client.Get(ctx, client.ObjectKey{Namespace: feed.Namespace, Name: jobName}, job)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err == nil {
			// Delete the existing job and return. When it's deleted, this Feed
			// will be reconciled again.
			glog.Infof("Found existing start job: %s/%s", job.Namespace, job.Name)

			// Need to delete pods first to workaround the client's lack of support
			// for cascading deletes. TODO remove this when client support allows.
			if err = r.deleteJobPods(ctx, job); err != nil {
				return err
			}
			r.client.Delete(ctx, job)
			glog.Infof("Deleted start job: %s/%s", job.Namespace, job.Name)
			return nil
		}

		jobName = resources.JobName(feed)

		if err := r.client.Get(ctx, client.ObjectKey{Namespace: feed.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(ctx, feed, es, et)
				if err != nil {
					return err
				}
				r.recorder.Eventf(feed, corev1.EventTypeNormal, "StopJobCreated", "Created stop job %q", job.Name)
				//TODO check for event source not found and remove finalizer

				feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
					Type:    feedsv1alpha1.FeedConditionReady,
					Status:  corev1.ConditionUnknown,
					Reason:  "StopJob",
					Message: "stop job in progress",
				})
			}
		}

		if resources.IsJobComplete(job) {
			r.recorder.Eventf(feed, corev1.EventTypeNormal, "StopJobCompleted", "Stop job %q completed", job.Name)
			feed.RemoveFinalizer(finalizerName)
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionReady,
				Status:  corev1.ConditionTrue,
				Reason:  "FeedSuccess",
				Message: "stop job succeeded",
			})
		} else if resources.IsJobFailed(job) {
			r.recorder.Eventf(feed, corev1.EventTypeWarning, "StopJobFailed", "Stop job %q failed: %q", job.Name, resources.JobFailedMessage(job))
			glog.Warningf("Stop job %q failed, removing finalizer on feed %q anyway.", job.Name, feed.Name)
			feed.RemoveFinalizer(finalizerName)
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionReady,
				Status:  corev1.ConditionFalse,
				Reason:  "FeedFailed",
				Message: fmt.Sprintf("Job failed with %s", resources.JobFailedMessage(job)),
			})
		}
	}
	return nil
}

// updateFeed updates the given Feed's owner references, finalizers, and status
// to the given values. It skips the update if none of those values would change.
func (r *reconciler) updateFeed(ctx context.Context, u *feedsv1alpha1.Feed) error {
	feed := &feedsv1alpha1.Feed{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, feed)
	if err != nil {
		return err
	}

	updated := false
	if !equality.Semantic.DeepEqual(feed.OwnerReferences, u.OwnerReferences) {
		feed.SetOwnerReferences(u.ObjectMeta.OwnerReferences)
		updated = true
	}

	if !equality.Semantic.DeepEqual(feed.Finalizers, u.Finalizers) {
		feed.SetFinalizers(u.ObjectMeta.Finalizers)
		updated = true
	}

	if !equality.Semantic.DeepEqual(feed.Status, u.Status) {
		feed.Status = u.Status
		updated = true
	}

	if updated == false {
		return nil
	}
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Feed resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return r.client.Update(ctx, feed)
}

// getEventTypeName returns the name of the Feed's referenced EventType or
// ClusterEventType.
func (r *reconciler) getEventTypeName(feed *feedsv1alpha1.Feed) string {
	if len(feed.Spec.Trigger.EventType) > 0 {
		return feed.Spec.Trigger.EventType
	} else if len(feed.Spec.Trigger.ClusterEventType) > 0 {
		return feed.Spec.Trigger.ClusterEventType
	}
	return ""
}

// setEventTypeOwnerReference makes the given Feed's referenced EventType or
// ClusterEventType a non-controlling owner of the Feed.
func (r *reconciler) setEventTypeOwnerReference(ctx context.Context, feed *feedsv1alpha1.Feed) error {
	// TODO(nicholss): need to set the owner on a cluser level event type as well.
	if len(feed.Spec.Trigger.EventType) > 0 {
		return r.setNamespacedEventTypeOwnerReference(ctx, feed)
	} else if len(feed.Spec.Trigger.ClusterEventType) > 0 {
		return r.setClusterEventTypeOwnerReference(ctx, feed)
	}
	return nil
}

// setEventTypeOwnerReference makes the given Feed's referenced EventType a
// non-controlling owner of the Feed.
func (r *reconciler) setNamespacedEventTypeOwnerReference(ctx context.Context, feed *feedsv1alpha1.Feed) error {
	et := &feedsv1alpha1.EventType{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: feed.Namespace, Name: feed.Spec.Trigger.EventType}, et); err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("Feed EventType not found, will not set finalizer")
			return nil
		}
		return err
	}

	blockOwnerDeletion := true
	isController := false
	ref := metav1.NewControllerRef(et, feedsv1alpha1.SchemeGroupVersion.WithKind("EventType"))
	ref.BlockOwnerDeletion = &blockOwnerDeletion
	ref.Controller = &isController

	feed.SetOwnerReference(ref)
	return nil
}

// setEventTypeOwnerReference makes the given Feed's referenced ClusterEventType
// a non-controlling owner of the Feed.
func (r *reconciler) setClusterEventTypeOwnerReference(ctx context.Context, feed *feedsv1alpha1.Feed) error {
	et := &feedsv1alpha1.ClusterEventType{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: feed.Spec.Trigger.ClusterEventType}, et); err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("Feed ClusterEventType not found, will not set finalizer")
			return nil
		}
		return err
	}

	blockOwnerDeletion := true
	isController := false
	ref := metav1.NewControllerRef(et, feedsv1alpha1.SchemeGroupVersion.WithKind("ClusterEventType"))
	ref.BlockOwnerDeletion = &blockOwnerDeletion
	ref.Controller = &isController

	feed.SetOwnerReference(ref)
	return nil
}

// resolveTrigger extracts the trigger from the Feed, reifies the parameters,
// and turns it all into an EventTrigger.
func (r *reconciler) resolveTrigger(ctx context.Context, feed *feedsv1alpha1.Feed) (sources.EventTrigger, error) {
	trigger := feed.Spec.Trigger
	resolved := sources.EventTrigger{
		Resource:   trigger.Resource,
		EventType:  r.getEventTypeName(feed),
		Parameters: make(map[string]interface{}),
	}

	if trigger.Parameters != nil && trigger.Parameters.Raw != nil && len(trigger.Parameters.Raw) > 0 {
		p := make(map[string]interface{})
		if err := yaml.Unmarshal(trigger.Parameters.Raw, &p); err != nil {
			return resolved, err
		}
		for k, v := range p {
			resolved.Parameters[k] = v
		}
	}
	if trigger.ParametersFrom != nil {
		glog.Infof("fetching from source %+v", trigger.ParametersFrom)
		for _, p := range trigger.ParametersFrom {
			pfs, err := r.fetchParametersFromSource(ctx, feed.Namespace, &p)
			if err != nil {
				return resolved, err
			}
			for k, v := range pfs {
				resolved.Parameters[k] = v
			}
		}
	}
	return resolved, nil
}

// fetchParametersFromSource gets the secret value referenced by the given
// ParametersFrom and returns it as a string-keyed map.
func (r *reconciler) fetchParametersFromSource(ctx context.Context, namespace string, parametersFrom *feedsv1alpha1.ParametersFromSource) (map[string]interface{}, error) {
	var params map[string]interface{}
	if parametersFrom.SecretKeyRef != nil {
		glog.Infof("fetching secret %+v", parametersFrom.SecretKeyRef)
		data, err := r.fetchSecretKeyValue(ctx, namespace, parametersFrom.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(data, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameters as JSON object: %v", err)
		}
	}
	return params, nil
}

// fetchSecretKeyValue gets the Secret referenced and returns the data in the
// referenced key.
func (r *reconciler) fetchSecretKeyValue(ctx context.Context, namespace string, secretKeyRef *feedsv1alpha1.SecretKeyReference) ([]byte, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretKeyRef.Name}, secret)
	if err != nil {
		return nil, err
	}
	return secret.Data[secretKeyRef.Key], nil
}

// createJob creates a Job for the given Feed based on its current state,
// returning the created Job.
func (r *reconciler) createJob(ctx context.Context, feed *feedsv1alpha1.Feed, es *feedsv1alpha1.EventSource, et *feedsv1alpha1.EventType) (*batchv1.Job, error) {
	trigger, err := r.resolveTrigger(ctx, feed)
	if err != nil {
		return nil, err
	}

	job, err := resources.MakeJob(feed, es, trigger, feed.Spec.Action.DNSName)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// setFeedContext sets the Feed's context from the context emitted by the given
// Job.
func (r *reconciler) setFeedContext(ctx context.Context, feed *feedsv1alpha1.Feed, job *batchv1.Job) error {
	feedContext, err := r.getJobContext(ctx, job)
	if err != nil {
		return err
	}

	marshalledFeedContext, err := json.Marshal(&feedContext.Context)
	if err != nil {
		return err
	}
	feed.Status.FeedContext = &runtime.RawExtension{
		Raw: marshalledFeedContext,
	}

	return nil
}

// getJobContext returns the FeedContext emitted by the first successful pod
// owned by this job. The feed context is extracted from the termination
// message of the first container in the pod.
func (r *reconciler) getJobContext(ctx context.Context, job *batchv1.Job) (*sources.FeedContext, error) {
	pods, err := r.getJobPods(ctx, job)
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		if p.Status.Phase == corev1.PodSucceeded {
			glog.Infof("Pod succeeded: %s", p.Name)
			if msg := resources.GetFirstTerminationMessage(&p); msg != "" {
				decodedContext, _ := base64.StdEncoding.DecodeString(msg)
				glog.Infof("decoded to %q", decodedContext)
				var ret sources.FeedContext
				err = json.Unmarshal(decodedContext, &ret)
				if err != nil {
					glog.Errorf("failed to unmarshal context: %s", err)
					return nil, err
				}
				return &ret, nil
			}
		}
	}
	return &sources.FeedContext{}, nil
}

// getJobPods returns the array of Pods owned by the given Job.
func (r *reconciler) getJobPods(ctx context.Context, job *batchv1.Job) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	listOptions := client.
		InNamespace(job.Namespace).
		MatchingLabels(job.Spec.Selector.MatchLabels)

	//TODO this is here because the fake client needs it. Remove this when it's
	// no longer needed.
	listOptions.Raw = &metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
	}

	if err := r.client.List(ctx, listOptions, podList); err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (r *reconciler) deleteJobPods(ctx context.Context, job *batchv1.Job) error {
	pods, err := r.getJobPods(ctx, job)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		if err := r.client.Delete(ctx, &pod); err != nil {
			return err
		}
		glog.Infof("Deleted start job pod: %s/%s", pod.Namespace, pod.Name)
	}
	return nil
}

// getFeedSource gets the EventSource and EventType that the trigger is targeting and
// returns them. If either the source or type is not found or is in the deleting state
// returns an error of the proper type.
func (r *reconciler) getFeedSource(ctx context.Context, feed *feedsv1alpha1.Feed) (*feedsv1alpha1.EventSource, *feedsv1alpha1.EventType, error) {
	es := &feedsv1alpha1.EventSource{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: feed.Namespace, Name: feed.Spec.Trigger.Service}, es); err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("EventSource %s/%s does not exist", feed.Namespace, feed.Spec.Trigger.Service)
			glog.Info(msg)
			return nil, nil, &EventSourceError{StatusError{EventSourceDoesNotExist, msg}}
		}
		return nil, nil, err
	}

	if es.GetDeletionTimestamp() != nil {
		// EventSource is being deleted so don't allow feeds to it
		msg := fmt.Sprintf("EventSource %s/%s is being deleted", feed.Namespace, feed.Spec.Trigger.Service)
		glog.Info(msg)
		return nil, nil, &EventSourceError{StatusError{EventSourceDeleting, msg}}
	}

	et := &feedsv1alpha1.EventType{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: feed.Namespace, Name: feed.Spec.Trigger.EventType}, et); err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("EventType %s/%s does not exist", feed.Namespace, feed.Spec.Trigger.EventType)
			glog.Info(msg)
			return nil, nil, &EventTypeError{StatusError{EventTypeDoesNotExist, msg}}
		}
		return nil, nil, err
	}

	if et.GetDeletionTimestamp() != nil {
		// EventType is being deleted so don't allow feeds to it
		msg := fmt.Sprintf("EventType %s/%s is being deleted", feed.Namespace, feed.Spec.Trigger.EventType)
		glog.Info(msg)
		return nil, nil, &EventTypeError{StatusError{EventTypeDeleting, msg}}
	}
	return es, et, nil
}
