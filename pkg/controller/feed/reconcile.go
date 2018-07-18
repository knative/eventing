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
	"strings"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/eventing/pkg/controller/feed/resources"
	"github.com/knative/eventing/pkg/sources"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
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

// Reconcile Feed resources
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	feed := &feedsv1alpha1.Feed{}
	err := r.client.Get(context.TODO(), request.NamespacedName, feed)

	if errors.IsNotFound(err) {
		glog.Errorf("could not find Feed %v\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		glog.Errorf("could not fetch Feed %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	r.setEventTypeOwnerReference(feed)
	if err := r.updateOwnerReferences(feed); err != nil {
		glog.Errorf("failed to update Feed owner references: %v", err)
		return reconcile.Result{}, err
	}

	feed.Status.InitializeConditions()
	if feed.GetDeletionTimestamp() == nil {
		err = r.reconcileStartJob(feed)
		if err != nil {
			glog.Errorf("error reconciling start Job: %v", err)
		}
	} else {
		err = r.reconcileStopJob(feed)
		if err != nil {
			glog.Errorf("error reconciling stop Job: %v", err)
		}
	}

	if err := r.updateStatus(feed); err != nil {
		glog.Errorf("failed to update Feed status: %v", err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *reconciler) reconcileStartJob(feed *feedsv1alpha1.Feed) error {
	bc := feed.Status.GetCondition(feedsv1alpha1.FeedConditionReady)
	switch bc.Status {
	case corev1.ConditionUnknown:

		job := &batchv1.Job{}
		jobName := resources.JobName(feed)

		if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: feed.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(feed)
				if err != nil {
					return err
				}
				feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
					Type:    feedsv1alpha1.FeedConditionReady,
					Status:  corev1.ConditionUnknown,
					Reason:  "StartJob",
					Message: "start job in progress",
				})
			}
		}
		feed.AddFinalizer(finalizerName)
		if err := r.updateFinalizers(feed); err != nil {
			return err
		}

		if resources.IsJobComplete(job) {
			if err := r.setFeedContext(feed, job); err != nil {
				return err
			}
			//TODO just use a single Succeeded condition, like Build
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionReady,
				Status:  corev1.ConditionTrue,
				Reason:  "FeedSuccess",
				Message: "start job succeeded",
			})
		} else if resources.IsJobFailed(job) {
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

func (r *reconciler) reconcileStopJob(feed *feedsv1alpha1.Feed) error {
	if feed.HasFinalizer(finalizerName) {

		// check for an existing start Job
		job := &batchv1.Job{}
		jobName := resources.StartJobName(feed)

		err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: feed.Namespace, Name: jobName}, job)
		if !errors.IsNotFound(err) {
			if err == nil {
				// Delete the existing job and return. When it's deleted, this Feed
				// will be reconciled again.
				r.client.Delete(context.TODO(), job)
				return nil
			}
			return err
		}

		jobName = resources.JobName(feed)

		if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: feed.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(feed)
				if err != nil {
					return err
				}
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
			feed.RemoveFinalizer(finalizerName)
			if err := r.updateFinalizers(feed); err != nil {
				return err
			}
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionReady,
				Status:  corev1.ConditionTrue,
				Reason:  "StopJobComplete",
				Message: "stop job succeeded",
			})
		} else if resources.IsJobFailed(job) {
			// finalizer remains to allow humans to inspect the failure
			feed.Status.SetCondition(&feedsv1alpha1.FeedCondition{
				Type:    feedsv1alpha1.FeedConditionReady,
				Status:  corev1.ConditionFalse,
				Reason:  "StopJobFailed",
				Message: fmt.Sprintf("Job failed with %s", resources.JobFailedMessage(job)),
			})
		}
	}
	return nil
}

func (r *reconciler) updateOwnerReferences(u *feedsv1alpha1.Feed) error {
	feed := &feedsv1alpha1.Feed{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, feed)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(feed.OwnerReferences, u.OwnerReferences) {
		feed.SetOwnerReferences(u.ObjectMeta.OwnerReferences)
		return r.client.Update(context.TODO(), feed)
	}
	return nil
}

func (r *reconciler) updateFinalizers(u *feedsv1alpha1.Feed) error {
	feed := &feedsv1alpha1.Feed{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, feed)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(feed.Finalizers, u.Finalizers) {
		feed.SetFinalizers(u.ObjectMeta.Finalizers)
		return r.client.Update(context.TODO(), feed)
	}
	return nil
}

func (r *reconciler) updateStatus(u *feedsv1alpha1.Feed) error {
	feed := &feedsv1alpha1.Feed{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, feed)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(feed.Status, u.Status) {
		feed.Status = u.Status
		// Until #38113 is merged, we must use Update instead of UpdateStatus to
		// update the Status block of the Feed resource. UpdateStatus will not
		// allow changes to the Spec of the resource, which is ideal for ensuring
		// nothing other than resource status has been updated.
		return r.client.Update(context.TODO(), feed)
	}
	return nil
}

func (r *reconciler) setEventTypeOwnerReference(feed *feedsv1alpha1.Feed) error {

	et := &feedsv1alpha1.EventType{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: feed.Namespace, Name: feed.Spec.Trigger.EventType}, et); err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("Feed event type not found, will not set finalizer")
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

func (r *reconciler) resolveTrigger(feed *feedsv1alpha1.Feed) (sources.EventTrigger, error) {
	trigger := feed.Spec.Trigger
	resolved := sources.EventTrigger{
		Resource:   trigger.Resource,
		EventType:  trigger.EventType,
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
			pfs, err := r.fetchParametersFromSource(feed.Namespace, &p)
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

func (r *reconciler) fetchParametersFromSource(namespace string, parametersFrom *feedsv1alpha1.ParametersFromSource) (map[string]interface{}, error) {
	var params map[string]interface{}
	if parametersFrom.SecretKeyRef != nil {
		glog.Infof("fetching secret %+v", parametersFrom.SecretKeyRef)
		data, err := r.fetchSecretKeyValue(namespace, parametersFrom.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		p, err := unmarshalJSON(data)
		if err != nil {
			return nil, err
		}
		params = p

	}
	return params, nil
}

func (r *reconciler) fetchSecretKeyValue(namespace string, secretKeyRef *feedsv1alpha1.SecretKeyReference) ([]byte, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: secretKeyRef.Name}, secret)
	if err != nil {
		return nil, err
	}
	return secret.Data[secretKeyRef.Key], nil
}

func unmarshalJSON(in []byte) (map[string]interface{}, error) {
	parameters := make(map[string]interface{})
	if err := json.Unmarshal(in, &parameters); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parameters as JSON object: %v", err)
	}
	return parameters, nil
}

func (r *reconciler) createJob(feed *feedsv1alpha1.Feed) (*batchv1.Job, error) {
	//TODO just use service dns
	target, err := r.resolveActionTarget(feed)
	if err != nil {
		return nil, err
	}

	source := &feedsv1alpha1.EventSource{}
	if err = r.client.Get(context.TODO(), client.ObjectKey{Namespace: feed.Namespace, Name: feed.Spec.Trigger.Service}, source); err != nil {
		return nil, err
	}

	et := &feedsv1alpha1.EventType{}
	if err = r.client.Get(context.TODO(), client.ObjectKey{Namespace: feed.Namespace, Name: feed.Spec.Trigger.EventType}, et); err != nil {
		return nil, err
	}

	trigger, err := r.resolveTrigger(feed)
	if err != nil {
		return nil, err
	}

	job, err := resources.MakeJob(feed, source, trigger, target)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(context.TODO(), job); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *reconciler) resolveActionTarget(feed *feedsv1alpha1.Feed) (string, error) {
	action := feed.Spec.Action
	if len(action.RouteName) > 0 {
		return r.resolveRouteDNS(feed.Namespace, action.RouteName)
	}
	if len(action.ChannelName) > 0 {
		return r.resolveChannelDNS(feed.Namespace, action.ChannelName)
	}
	// This should never happen, but because we don't have webhook validation yet, check
	// and complain.
	return "", fmt.Errorf("action is missing both RouteName and ChannelName")
}

func (r *reconciler) resolveRouteDNS(namespace string, routeName string) (string, error) {
	route := &servingv1alpha1.Route{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: routeName}, route)
	if err != nil {
		return "", err
	}
	if len(route.Status.Domain) == 0 {
		return "", fmt.Errorf("route '%s/%s' is missing a domain", namespace, routeName)
	}
	return route.Status.Domain, nil
}

func (r *reconciler) resolveChannelDNS(namespace string, channelName string) (string, error) {
	// Currently Flow object resolves the channel to the DNS so allow for a fully
	// specified cluster name to be returned as is.
	// TODO: once the FeedAction gets normalized, clean this up.
	if strings.HasSuffix(channelName, "svc.cluster.local") {
		glog.Infof("using channel name as DNS entry: %q", channelName)
		return channelName, nil
	}

	channel := &channelsv1alpha1.Channel{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: channelName}, channel)
	if err != nil {
		return "", err
	}

	if channel.Status.DomainInternal != "" {
		return channel.Status.DomainInternal, nil
	}
	return "", fmt.Errorf("channel '%s/%s' does not have Status.DomainInternal", namespace, channelName)
}

func (r *reconciler) setFeedContext(feed *feedsv1alpha1.Feed, job *batchv1.Job) error {
	ctx, err := r.getJobContext(job)
	if err != nil {
		return err
	}

	marshalledFeedContext, err := json.Marshal(&ctx.Context)
	if err != nil {
		return err
	}
	feed.Status.FeedContext = &runtime.RawExtension{
		Raw: marshalledFeedContext,
	}

	return nil
}

func (r *reconciler) getJobContext(job *batchv1.Job) (*sources.FeedContext, error) {
	pods, err := r.getJobPods(job)
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

func (r *reconciler) getJobPods(job *batchv1.Job) ([]corev1.Pod, error) {
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

	if err := r.client.List(context.TODO(), listOptions, podList); err != nil {
		return nil, err
	}

	return podList.Items, nil
}
