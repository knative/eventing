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

package subscription

import (
	"context"
	"fmt"
	"log"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/apis/duck/util"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Subscription resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Reconciling subscription %v", request)
	subscription := &v1alpha1.Subscription{}
	err := r.client.Get(context.TODO(), request.NamespacedName, subscription)

	if errors.IsNotFound(err) {
		glog.Errorf("could not find subscription %v\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		glog.Errorf("could not fetch Subscription %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	original := subscription.DeepCopy()

	// Reconcile this copy of the Subscription and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(subscription)
	if equality.Semantic.DeepEqual(original.Status, subscription.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := r.updateStatus(subscription); err != nil {
		glog.Warningf("Failed to update subscription status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(subscription *v1alpha1.Subscription) error {
	// See if the subscription has been deleted
	accessor, err := meta.Accessor(subscription)
	if err != nil {
		log.Fatalf("Failed to get metadata: %s", err)
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	glog.Infof("DeletionTimestamp: %v", deletionTimestamp)

	// First resolve the From channel
	from, err := r.resolveFromChannelable(subscription.Namespace, &subscription.Spec.From)
	if err != nil {
		glog.Warningf("Failed to resolve From %v : %v", subscription.Spec.From, err)
		return err
	}
	glog.Infof("Resolved From channel to Channelable %+v", *from)

	if subscription.Spec.Call != nil {
		call, err := r.resolveCall(subscription.Namespace, *subscription.Spec.Call)
		if err != nil {
			glog.Warningf("Failed to resolve Call %v : %v", *subscription.Spec.Call, err)
		}
		glog.Infof("Resolved call to: %q", call)
	}

	if subscription.Spec.Result != nil {
		result, err := r.resolveResult(subscription.Namespace, *subscription.Spec.Result)
		if err != nil {
			glog.Warningf("Failed to resolve Result %v : %v", subscription.Spec.Result, err)
		}
		glog.Infof("Resolved result to: %q", result)
	}

	// Reconcile the subscription to the From channel that's consuming events that are either
	// going to the call or if there's no call, directly to result.
	// TODO: deal with deletion and remove the subscription
	// Targetable and Sinkable have different transport contracts so it's little tricky...
	// TODO: WIP: What's the right abstraction here. I'm fairly certain it is not the
	// Call/Result pair because I think it's something that should be resolved at this
	// level and subscription should not know wheether there's a Call or a Result. I think
	// the right abstraction is Sinkable, since it has the contract that it either accepts
	// teh event for further pcessing (details be damned) or it doesnt

	return nil
}

func (r *reconciler) updateStatus(subscription *v1alpha1.Subscription) (*v1alpha1.Subscription, error) {
	newSubscription := &v1alpha1.Subscription{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: subscription.Namespace, Name: subscription.Name}, newSubscription)

	if err != nil {
		return nil, err
	}
	newSubscription.Status = subscription.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Subscription resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err = r.client.Update(context.TODO(), newSubscription); err != nil {
		return nil, err
	}
	return newSubscription, nil
}

// resolveCall resolves the Spec.Call object. If it's an ObjectReference will resolve the object
// and treat it as a Targetable.If it's TargetURI then it's used as is.
func (r *reconciler) resolveCall(namespace string, callable v1alpha1.Callable) (string, error) {
	if callable.TargetURI != nil && *callable.TargetURI != "" {
		return *callable.TargetURI, nil
	}

	obj, err := r.fetchObjectReference(namespace, callable.Target)
	if err != nil {
		glog.Warningf("Feiled to fetch Callable target %+v: %s", callable.Target, err)
		return "", err
	}
	t := duckv1alpha1.Target{}
	err = util.FromUnstructured(*obj, &t)
	if err != nil {
		glog.Warningf("Feiled to unserialize target: %s", err)
		return "", err
	}
	if t.Status.Targetable != nil {
		return t.Status.Targetable.DomainInternal, nil
	}
	return "", fmt.Errorf("status does not contain targetable")
}

// resolveResult resolves the Spec.Result object.
func (r *reconciler) resolveResult(namespace string, resultStrategy v1alpha1.ResultStrategy) (string, error) {
	obj, err := r.fetchObjectReference(namespace, resultStrategy.Target)
	if err != nil {
		glog.Warningf("Feiled to fetch ResultStrategy target %+v: %s", resultStrategy, err)
		return "", err
	}
	s := duckv1alpha1.Sink{}
	err = util.FromUnstructured(*obj, &s)
	if err != nil {
		glog.Warningf("Feiled to unserialize Sinkable target: %s", err)
		return "", err
	}
	if s.Status.Sinkable != nil {
		return s.Status.Sinkable.DomainInternal, nil
	}
	return "", fmt.Errorf("status does not contain targetable")
}

// resolveFromChannelable fetches an object based on ObjectReference. It assumes that the
// fetched object then implements Subscribable interface and returns the ObjectReference
// representing the Channelable interface.
func (r *reconciler) resolveFromChannelable(namespace string, ref *corev1.ObjectReference) (*duckv1alpha1.Subscription, error) {
	obj, err := r.fetchObjectReference(namespace, ref)
	if err != nil {
		glog.Warningf("Feiled to fetch ResultStrategy target %+v: %s", ref, err)
		return nil, err
	}

	c := duckv1alpha1.Subscription{}
	err = util.FromUnstructured(*obj, &c)
	return &c, err
}

// fetchObjectReference fetches an object based on ObjectReference.
func (r *reconciler) fetchObjectReference(namespace string, ref *corev1.ObjectReference) (*unstructured.Unstructured, error) {
	resourceClient, err := CreateResourceInterface(r.restConfig, ref, namespace)
	if err != nil {
		glog.Warningf("failed to create dynamic client resource: %v", err)
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

const (
	// controllerConfigMapName is the name of the configmap in the eventing
	// namespace that holds the configuration for this controller.
	controllerConfigMapName = "subscription-controller-config"

	// defaultClusterBusConfigMapKey is the name of the key in this controller's
	// ConfigMap that contains the name of the default cluster bus for the subscription
	// controller to use.
	defaultClusterBusConfigMapKey = "default-cluster-bus"

	// fallbackClusterBusName is the name of the cluster bus that will be used
	// for subscriptions if the controller's configmap does not exist or does not
	// contain the 'default-cluster-bus' key.
	fallbackClusterBusName = "stub"
)
