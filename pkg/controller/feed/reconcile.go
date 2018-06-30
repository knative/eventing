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

package bind

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/eventing/pkg/controller/bind/resources"
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

// Reconcile Routes
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	bind := &feedsv1alpha1.Bind{}
	err := r.client.Get(context.TODO(), request.NamespacedName, bind)

	if errors.IsNotFound(err) {
		glog.Errorf("Could not find Bind %v\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		glog.Errorf("Could not fetch Bind %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	r.setEventTypeOwnerReference(bind)
	if err := r.updateOwnerReferences(bind); err != nil {
		glog.Errorf("Failed to update Bind owner references: %v", err)
		return reconcile.Result{}, err
	}

	bind.Status.InitializeConditions()
	if bind.GetDeletionTimestamp() == nil {
		err = r.reconcileBindJob(bind)
	} else {
		err = r.reconcileUnbindJob(bind)
	}
	if err != nil {
		glog.Errorf("Error reconciling bind job: %v", err)
	}

	if err := r.updateStatus(bind); err != nil {
		glog.Errorf("Failed to update Bind status: %v", err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *reconciler) reconcileBindJob(bind *feedsv1alpha1.Bind) error {
	bc := bind.Status.GetCondition(feedsv1alpha1.BindComplete)
	switch bc.Status {
	case corev1.ConditionUnknown:

		job := &batchv1.Job{}
		jobName := resources.JobName(bind)

		if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: bind.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(bind)
				if err != nil {
					return err
				}
				bind.Status.SetCondition(&feedsv1alpha1.BindCondition{
					Type:    feedsv1alpha1.BindComplete,
					Status:  corev1.ConditionUnknown,
					Reason:  "BindJob",
					Message: "Bind job in progress",
				})
			}
		}
		bind.AddFinalizer(finalizerName)
		if err := r.updateFinalizers(bind); err != nil {
			return err
		}

		if resources.IsJobComplete(job) {
			if err := r.setBindContext(bind, job); err != nil {
				return err
			}
			//TODO just use a single Succeeded condition, like Build
			bind.Status.SetCondition(&feedsv1alpha1.BindCondition{
				Type:    feedsv1alpha1.BindComplete,
				Status:  corev1.ConditionTrue,
				Reason:  "BindJobComplete",
				Message: "Bind job succeeded",
			})
		} else if resources.IsJobFailed(job) {
			bind.Status.SetCondition(&feedsv1alpha1.BindCondition{
				Type:    feedsv1alpha1.BindFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "BindJobFailed",
				Message: "TODO replace with job failure message",
			})
		}

	case corev1.ConditionTrue:
		//TODO delete job
	}
	return nil
}

func (r *reconciler) reconcileUnbindJob(bind *feedsv1alpha1.Bind) error {
	if bind.HasFinalizer(finalizerName) {

		// check for an existing bind job
		job := &batchv1.Job{}
		jobName := resources.BindJobName(bind)

		err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: bind.Namespace, Name: jobName}, job)
		if !errors.IsNotFound(err) {
			if err == nil {
				// Delete the existing job and return. When it's deleted, this Bind
				// will be reconciled again.
				r.client.Delete(context.TODO(), job)
				return nil
			}
			return err
		}

		jobName = resources.JobName(bind)

		if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: bind.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(bind)
				if err != nil {
					return err
				}
				//TODO check for event source not found and remove finalizer

				bind.Status.SetCondition(&feedsv1alpha1.BindCondition{
					Type:    feedsv1alpha1.BindComplete,
					Status:  corev1.ConditionUnknown,
					Reason:  "UnbindJob",
					Message: "Unbind job in progress",
				})
			}
		}

		if resources.IsJobComplete(job) {
			bind.RemoveFinalizer(finalizerName)
			if err := r.updateFinalizers(bind); err != nil {
				return err
			}
			bind.Status.SetCondition(&feedsv1alpha1.BindCondition{
				Type:    feedsv1alpha1.BindComplete,
				Status:  corev1.ConditionTrue,
				Reason:  "UnbindJobComplete",
				Message: "Unbind job succeeded",
			})
		} else if resources.IsJobFailed(job) {
			// finalizer remains to allow humans to inspect the failure
			bind.Status.SetCondition(&feedsv1alpha1.BindCondition{
				Type:    feedsv1alpha1.BindFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "UnbindJobFailed",
				Message: "TODO replace with job failure message",
			})
		}
	}
	return nil
}

func (r *reconciler) updateOwnerReferences(u *feedsv1alpha1.Bind) error {
	bind := &feedsv1alpha1.Bind{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, bind)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(bind.OwnerReferences, u.OwnerReferences) {
		bind.SetOwnerReferences(u.ObjectMeta.OwnerReferences)
		return r.client.Update(context.TODO(), bind)
	}
	return nil
}

func (r *reconciler) updateFinalizers(u *feedsv1alpha1.Bind) error {
	bind := &feedsv1alpha1.Bind{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, bind)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(bind.Finalizers, u.Finalizers) {
		bind.SetFinalizers(u.ObjectMeta.Finalizers)
		return r.client.Update(context.TODO(), bind)
	}
	return nil
}

func (r *reconciler) updateStatus(u *feedsv1alpha1.Bind) error {
	bind := &feedsv1alpha1.Bind{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, bind)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(bind.Status, u.Status) {
		bind.Status = u.Status
		// Until #38113 is merged, we must use Update instead of UpdateStatus to
		// update the Status block of the Bind resource. UpdateStatus will not
		// allow changes to the Spec of the resource, which is ideal for ensuring
		// nothing other than resource status has been updated.
		return r.client.Update(context.TODO(), bind)
	}
	return nil
}

func (r *reconciler) setEventTypeOwnerReference(bind *feedsv1alpha1.Bind) error {

	et := &feedsv1alpha1.EventType{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: bind.Namespace, Name: bind.Spec.Trigger.EventType}, et); err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("Bind event type not found, will not set finalizer")
			return nil
		}
		return err
	}

	blockOwnerDeletion := true
	isController := false
	ref := metav1.NewControllerRef(et, feedsv1alpha1.SchemeGroupVersion.WithKind("EventType"))
	ref.BlockOwnerDeletion = &blockOwnerDeletion
	ref.Controller = &isController

	bind.SetOwnerReference(ref)
	return nil
}

func (r *reconciler) resolveTrigger(bind *feedsv1alpha1.Bind) (sources.EventTrigger, error) {
	trigger := bind.Spec.Trigger
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
		glog.Infof("Fetching from source %+v", trigger.ParametersFrom)
		for _, p := range trigger.ParametersFrom {
			pfs, err := r.fetchParametersFromSource(bind.Namespace, &p)
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
		glog.Infof("Fetching secret %+v", parametersFrom.SecretKeyRef)
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

func (r *reconciler) createJob(bind *feedsv1alpha1.Bind) (*batchv1.Job, error) {
	//TODO just use service dns
	target, err := r.resolveActionTarget(bind)
	if err != nil {
		return nil, err
	}

	source := &feedsv1alpha1.EventSource{}
	if err = r.client.Get(context.TODO(), client.ObjectKey{Namespace: bind.Namespace, Name: bind.Spec.Trigger.Service}, source); err != nil {
		return nil, err
	}

	et := &feedsv1alpha1.EventType{}
	if err = r.client.Get(context.TODO(), client.ObjectKey{Namespace: bind.Namespace, Name: bind.Spec.Trigger.EventType}, et); err != nil {
		return nil, err
	}

	trigger, err := r.resolveTrigger(bind)
	if err != nil {
		return nil, err
	}

	job, err := resources.MakeJob(bind, source, trigger, target)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(context.TODO(), job); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *reconciler) resolveActionTarget(bind *feedsv1alpha1.Bind) (string, error) {
	action := bind.Spec.Action
	if len(action.RouteName) > 0 {
		return r.resolveRouteDNS(bind.Namespace, action.RouteName)
	}
	if len(action.ChannelName) > 0 {
		return r.resolveChannelDNS(bind.Namespace, action.ChannelName)
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
	channel := &channelsv1alpha1.Channel{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: channelName}, channel)
	if err != nil {
		return "", err
	}
	// TODO: The actual dns name should come from something in the status, or ?? But right
	// now it is hard coded to be <channelname>-channel
	// So we just check that the channel actually exists and tack on the -channel
	return fmt.Sprintf("%s-channel", channel.Name), nil
}

func (r *reconciler) setBindContext(bind *feedsv1alpha1.Bind, job *batchv1.Job) error {
	ctx, err := r.getJobContext(job)
	if err != nil {
		return err
	}

	marshalledBindContext, err := json.Marshal(&ctx.Context)
	if err != nil {
		return err
	}
	bind.Status.BindContext = &runtime.RawExtension{
		Raw: marshalledBindContext,
	}

	return nil
}

func (r *reconciler) getJobContext(job *batchv1.Job) (*sources.BindContext, error) {
	pods, err := r.getJobPods(job)
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		if p.Status.Phase == corev1.PodSucceeded {
			glog.Infof("Pod succeeded: %s", p.Name)
			if msg := resources.GetFirstTerminationMessage(&p); msg != "" {
				decodedContext, _ := base64.StdEncoding.DecodeString(msg)
				glog.Infof("Decoded to %q", decodedContext)
				var ret sources.BindContext
				err = json.Unmarshal(decodedContext, &ret)
				if err != nil {
					glog.Errorf("Failed to unmarshal context: %s", err)
					return nil, err
				}
				return &ret, nil
			}
		}
	}
	return &sources.BindContext{}, nil
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
