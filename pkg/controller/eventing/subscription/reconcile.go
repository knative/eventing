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
	"strings"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
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
	subscription.Status.InitializeConditions()

	// See if the subscription has been deleted
	accessor, err := meta.Accessor(subscription)
	if err != nil {
		glog.Warningf("Failed to get metadata accessor: %s", err)
		return err
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	glog.Infof("DeletionTimestamp: %v", deletionTimestamp)

	// Reconcile the subscription to the From channel that's consuming events that are either
	// going to the call or if there's no call, directly to result.
	from, err := r.resolveFromChannelable(subscription.Namespace, &subscription.Spec.From)
	if err != nil {
		glog.Warningf("Failed to resolve From %+v : %s", subscription.Spec.From, err)
		return err
	}
	if from.Status.Subscribable == nil {
		return fmt.Errorf("from is not subscribable %s %s/%s", subscription.Spec.From.Kind, subscription.Namespace, subscription.Spec.From.Name)
	}

	callDomain := ""
	if subscription.Spec.Call != nil {
		callDomain, err = r.resolveCall(subscription.Namespace, *subscription.Spec.Call)
		if err != nil {
			glog.Warningf("Failed to resolve Call %+v : %s", *subscription.Spec.Call, err)
			return err
		}
		if callDomain == "" {
			return fmt.Errorf("could not get domain from call (is it not targetable?)")
		}
		glog.Infof("Resolved call to: %q", callDomain)
	}

	resultDomain := ""
	if subscription.Spec.Result != nil {
		resultDomain, err = r.resolveResult(subscription.Namespace, *subscription.Spec.Result)
		if err != nil {
			glog.Warningf("Failed to resolve Result %v : %v", subscription.Spec.Result, err)
			return err
		}
		if resultDomain == "" {
			glog.Warningf("Failed to resolve result %v to actual domain", *subscription.Spec.Result)
			return err
		}
		glog.Infof("Resolved result to: %q", resultDomain)
	}

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	// Ok, now that we have the From and at least one of the Call/Result, let's reconcile
	// the From with this information.
	err = r.reconcileFromChannel(subscription.Namespace, from.Status.Subscribable.Channelable, callDomain, resultDomain, deletionTimestamp != nil)
	if err != nil {
		glog.Warningf("Failed to resolve from Channel : %s", err)
		return err
	}
	// Everything went well, set the fact that subscriptions have been modified
	subscription.Status.MarkFromReady()
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
// TODO: Once Service Routes, etc. support Targetable, use that.
//
func (r *reconciler) resolveCall(namespace string, callable v1alpha1.Callable) (string, error) {
	if callable.TargetURI != nil && *callable.TargetURI != "" {
		return *callable.TargetURI, nil
	}

	obj, err := r.fetchObjectReference(namespace, callable.Target)
	if err != nil {
		glog.Warningf("Failed to fetch Callable target %+v: %s", callable.Target, err)
		return "", err
	}
	t := duckv1alpha1.LegacyTarget{}
	// Once Knative services support Targetable, switch to using this.
	//t := duckv1alpha1.Target{}
	err = duck.FromUnstructured(*obj, &t)
	if err != nil {
		glog.Warningf("Failed to unserialize legacy target: %s", err)
		return "", err
	}

	return t.Status.DomainInternal, nil
	// Once Knative services support Targetable, switch to using this
	// 	if t.Status.Targetable != nil {
	//		return t.Status.Targetable.DomainInternal, nil
	//	}
	//return "", fmt.Errorf("status does not contain targetable")
}

// resolveResult resolves the Spec.Result object.
func (r *reconciler) resolveResult(namespace string, resultStrategy v1alpha1.ResultStrategy) (string, error) {
	obj, err := r.fetchObjectReference(namespace, resultStrategy.Target)
	if err != nil {
		glog.Warningf("Failed to fetch ResultStrategy target %+v: %s", resultStrategy, err)
		return "", err
	}
	s := duckv1alpha1.Sink{}
	err = duck.FromUnstructured(*obj, &s)
	if err != nil {
		glog.Warningf("Failed to unserialize Sinkable target: %s", err)
		return "", err
	}
	if s.Status.Sinkable != nil {
		return s.Status.Sinkable.DomainInternal, nil
	}
	return "", fmt.Errorf("status does not contain sinkable")
}

// resolveFromChannelable fetches an object based on ObjectReference. It assumes that the
// fetched object then implements Subscribable interface and returns the ObjectReference
// representing the Channelable interface.
func (r *reconciler) resolveFromChannelable(namespace string, ref *corev1.ObjectReference) (*duckv1alpha1.Subscription, error) {
	obj, err := r.fetchObjectReference(namespace, ref)
	if err != nil {
		glog.Warningf("Failed to fetch From target %+v: %s", ref, err)
		return nil, err
	}

	c := duckv1alpha1.Subscription{}
	err = duck.FromUnstructured(*obj, &c)
	return &c, err
}

// fetchObjectReference fetches an object based on ObjectReference.
func (r *reconciler) fetchObjectReference(namespace string, ref *corev1.ObjectReference) (*unstructured.Unstructured, error) {
	//	resourceClient, err := r.CreateResourceInterface2(r.restConfig, ref, namespace)
	resourceClient, err := r.CreateResourceInterface(namespace, ref)
	if err != nil {
		glog.Warningf("failed to create dynamic client resource: %v", err)
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

func (r *reconciler) reconcileFromChannel(namespace string, subscribable corev1.ObjectReference, callDomain string, resultDomain string, deleted bool) error {
	glog.Infof("Reconciling From Channel: %+v call: %q result %q deleted: %v", subscribable, callDomain, resultDomain, deleted)

	// First get the original object and convert it to only the bits we care about
	s, err := r.fetchObjectReference(namespace, &subscribable)
	if err != nil {
		return err
	}
	original := duckv1alpha1.Channel{}
	err = duck.FromUnstructured(*s, &original)
	if err != nil {
		return err
	}

	// TODO: Handle deletes.

	after := original.DeepCopy()
	after.Spec.Channelable = &duckv1alpha1.Channelable{
		Subscribers: []duckv1alpha1.ChannelSubscriberSpec{{CallableDomain: callDomain, SinkableDomain: resultDomain}},
	}

	patch, err := duck.CreatePatch(original, after)
	if err != nil {
		return err
	}

	patchBytes, err := patch.MarshalJSON()
	if err != nil {
		glog.Warningf("failed to marshal json patch: %s", err)
		return err
	}

	resourceClient, err := r.CreateResourceInterface(namespace, &subscribable)
	if err != nil {
		glog.Warningf("failed to create dynamic client resource: %v", err)
		return err
	}
	patched, err := resourceClient.Patch(original.Name, types.JSONPatchType, patchBytes)
	if err != nil {
		glog.Warningf("Failed to patch the object: %s", err)
		glog.Warningf("Patch was: %+v", patch)
		return err
	}
	glog.Warningf("Patched resource: %+v", patched)
	return nil
}

func (r *reconciler) CreateResourceInterface(namespace string, ref *corev1.ObjectReference) (dynamic.ResourceInterface, error) {
	gvk := ref.GroupVersionKind()

	rc := r.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: pluralizeKind(gvk.Kind),
	})
	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil

}

// takes a kind and pluralizes it. This is super terrible, but I am
// not aware of a generic way to do this.
// I am not alone in thinking this and I haven't found a better solution:
// This seems relevant:
// https://github.com/kubernetes/kubernetes/issues/18622
func pluralizeKind(kind string) string {
	ret := strings.ToLower(kind)
	if strings.HasSuffix(ret, "s") {
		return fmt.Sprintf("%ses", ret)
	}
	return fmt.Sprintf("%ss", ret)
}
