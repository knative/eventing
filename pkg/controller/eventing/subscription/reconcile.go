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
	"net/url"

	"github.com/golang/glog"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	duckapis "github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	} else if _, updateStatusErr := r.updateStatus(subscription); updateStatusErr != nil {
		glog.Warningf("Failed to update subscription status: %v", updateStatusErr)
		return reconcile.Result{}, updateStatusErr
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

	// Verify that `from` exists.
	_, err = r.fetchObjectReference(subscription.Namespace, &subscription.Spec.From)
	if err != nil {
		glog.Warningf("Failed to validate `from` exists: %+v, %v", subscription.Spec.From, err)
		return err
	}

	callURI := ""
	if subscription.Spec.Call != nil {
		callURI, err = r.resolveEndpointSpec(subscription.Namespace, *subscription.Spec.Call)
		if err != nil {
			glog.Warningf("Failed to resolve Call %+v : %s", *subscription.Spec.Call, err)
			return err
		}
		if callURI == "" {
			return fmt.Errorf("could not get domain from call (is it not targetable?)")
		}
		subscription.Status.PhysicalSubscription.CallURI = callURI
		glog.Infof("Resolved call to: %q", callURI)
	}

	resultURI := ""
	if subscription.Spec.Result != nil {
		resultURI, err = r.resolveResult(subscription.Namespace, *subscription.Spec.Result)
		if err != nil {
			glog.Warningf("Failed to resolve Result %v : %v", subscription.Spec.Result, err)
			return err
		}
		if resultURI == "" {
			glog.Warningf("Failed to resolve result %v to actual domain", *subscription.Spec.Result)
			return err
		}
		subscription.Status.PhysicalSubscription.ResultURI = resultURI
		glog.Infof("Resolved result to: %q", resultURI)
	}

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	// Ok, now that we have the From and at least one of the Call/Result, let's reconcile
	// the From with this information.
	err = r.syncPhysicalFromChannel(subscription)
	if err != nil {
		glog.Warningf("Failed to sync physical from Channel : %s", err)
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

// resolveEndpointSpec resolves the Spec.Call object. If it's an
// ObjectReference will resolve the object and treat it as a Targetable. If
// it's DNSName then it's used as is.
// TODO: Once Service Routes, etc. support Targetable, use that.
//
func (r *reconciler) resolveEndpointSpec(namespace string, es v1alpha1.EndpointSpec) (string, error) {
	if es.DNSName != nil && *es.DNSName != "" {
		return *es.DNSName, nil
	}

	// K8s services are special cased. They can be called, even though they do not satisfy the
	// Targetable interface.
	if es.TargetRef != nil && es.TargetRef.APIVersion == "v1" && es.TargetRef.Kind == "Service" {
		svc := &corev1.Service{}
		svcKey := types.NamespacedName{
			Namespace: namespace,
			Name:      es.TargetRef.Name,
		}
		err := r.client.Get(context.TODO(), svcKey, svc)
		if err != nil {
			glog.Warningf("Failed to fetch EndpointSpec target as a K8s Service %+v: %s", es.TargetRef, err)
			return "", err
		}
		return domainToURL(controller.ServiceHostName(svc.Name, svc.Namespace)), nil
	}

	obj, err := r.fetchObjectReference(namespace, es.TargetRef)
	if err != nil {
		glog.Warningf("Failed to fetch EndpointSpec target %+v: %s", es.TargetRef, err)
		return "", err
	}
	t := duckv1alpha1.Target{}
	err = duck.FromUnstructured(obj, &t)
	if err != nil {
		glog.Warningf("Failed to deserialize legacy target: %s", err)
		return "", err
	}

	if t.Status.Targetable != nil {
		return domainToURL(t.Status.Targetable.DomainInternal), nil
	}
	return "", fmt.Errorf("status does not contain targetable")
}

// resolveResult resolves the Spec.Result object.
func (r *reconciler) resolveResult(namespace string, resultStrategy v1alpha1.ResultStrategy) (string, error) {
	obj, err := r.fetchObjectReference(namespace, resultStrategy.Target)
	if err != nil {
		glog.Warningf("Failed to fetch ResultStrategy target %+v: %s", resultStrategy, err)
		return "", err
	}
	s := duckv1alpha1.Sink{}
	err = duck.FromUnstructured(obj, &s)
	if err != nil {
		glog.Warningf("Failed to deserialize Sinkable target: %s", err)
		return "", err
	}
	if s.Status.Sinkable != nil {
		return domainToURL(s.Status.Sinkable.DomainInternal), nil
	}
	return "", fmt.Errorf("status does not contain sinkable")
}

// fetchObjectReference fetches an object based on ObjectReference.
func (r *reconciler) fetchObjectReference(namespace string, ref *corev1.ObjectReference) (duck.Marshalable, error) {
	resourceClient, err := r.CreateResourceInterface(namespace, ref)
	if err != nil {
		glog.Warningf("failed to create dynamic client resource: %v", err)
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

func domainToURL(domain string) string {
	u := url.URL{
		Scheme: "http",
		Host:   domain,
		Path:   "/",
	}
	return u.String()
}

func (r *reconciler) syncPhysicalFromChannel(sub *v1alpha1.Subscription) error {
	glog.Infof("Reconciling Physical From Channel: %+v", sub)

	subs, err := r.listAllSubscriptionsWithPhysicalFrom(sub)
	if err != nil {
		glog.Infof("Unable to list all channels with physical from: %+v", err)
		return err
	}

	channelable := r.createChannelable(subs)

	return r.patchPhysicalFrom(sub.Namespace, sub.Spec.From, channelable)
}

func (r *reconciler) listAllSubscriptionsWithPhysicalFrom(sub *v1alpha1.Subscription) ([]v1alpha1.Subscription, error) {
	subs := make([]v1alpha1.Subscription, 0)

	// The sub we are currently reconciling has not yet written any updated status, so when listing
	// it won't show any updates to the Status.PhysicalSubscription. We know that we are listing
	// for subscriptions with the same PhysicalSubscription.From, so just add this one manually.
	subs = append(subs, *sub)

	opts := &client.ListOptions{
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "Subscription",
			},
		},
	}
	ctx := context.TODO()
	for {
		sl := &v1alpha1.SubscriptionList{}
		err := r.client.List(ctx, opts, sl)
		if err != nil {
			return nil, err
		}
		for _, s := range sl.Items {
			if sub.UID == s.UID {
				// This is the sub that is being reconciled. It has already been added to the list.
				continue
			}
			if equality.Semantic.DeepEqual(sub.Spec.From, s.Spec.From) {
				subs = append(subs, s)
			}
		}
		if sl.Continue != "" {
			opts.Raw.Continue = sl.Continue
		} else {
			return subs, nil
		}
	}
}

func (r *reconciler) createChannelable(subs []v1alpha1.Subscription) *eventingduck.Channelable {
	rv := &eventingduck.Channelable{}
	for _, sub := range subs {
		if sub.Status.PhysicalSubscription.CallURI != "" || sub.Status.PhysicalSubscription.ResultURI != "" {
			rv.Subscribers = append(rv.Subscribers, eventingduck.ChannelSubscriberSpec{
				CallableURI: sub.Status.PhysicalSubscription.CallURI,
				SinkableURI: sub.Status.PhysicalSubscription.ResultURI,
			})
		}
	}
	return rv
}

func (r *reconciler) patchPhysicalFrom(namespace string, physicalFrom corev1.ObjectReference, subs *eventingduck.Channelable) error {
	// First get the original object and convert it to only the bits we care about
	s, err := r.fetchObjectReference(namespace, &physicalFrom)
	if err != nil {
		return err
	}
	original := eventingduck.Channel{}
	err = duck.FromUnstructured(s, &original)
	if err != nil {
		return err
	}

	after := original.DeepCopy()
	after.Spec.Channelable = subs

	patch, err := duck.CreatePatch(original, after)
	if err != nil {
		return err
	}

	patchBytes, err := patch.MarshalJSON()
	if err != nil {
		glog.Warningf("failed to marshal json patch: %s", err)
		return err
	}

	resourceClient, err := r.CreateResourceInterface(namespace, &physicalFrom)
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
	rc := r.dynamicClient.Resource(duckapis.KindToResource(ref.GroupVersionKind()))

	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil

}
