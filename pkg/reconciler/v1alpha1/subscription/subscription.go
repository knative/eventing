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

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/golang/glog"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler/names"
	duckapis "github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "subscription-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process
	subscriptionReconciled         = "SubscriptionReconciled"
	subscriptionUpdateStatusFailed = "SubscriptionUpdateStatusFailed"
	physicalChannelSyncFailed      = "PhysicalChannelSyncFailed"
	channelReferenceFetchFailed    = "ChannelReferenceFetchFailed"
	subscriberResolveFailed        = "SubscriberResolveFailed"
	resultResolveFailed            = "ResultResolveFailed"
)

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a Subscription controller.
func ProvideController(mgr manager.Manager) (controller.Controller, error) {
	// Setup a new controller to Reconcile Subscriptions.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Subscription events and enqueue Subscription object key.
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Subscription{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	return c, nil
}

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

	// Reconcile this copy of the Subscription and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(subscription)
	if err != nil {
		glog.Warningf("Error reconciling Subscription: %v", err)
	} else {
		glog.Info("Subscription reconciled")
		r.recorder.Eventf(subscription, corev1.EventTypeNormal, subscriptionReconciled, "Subscription reconciled: %q", subscription.Name)
	}

	if _, updateStatusErr := r.updateStatus(subscription.DeepCopy()); updateStatusErr != nil {
		glog.Warningf("Failed to update subscription status: %v", updateStatusErr)
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, subscriptionUpdateStatusFailed, "Failed to update Subscription's status: %v", err)
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

	if subscription.DeletionTimestamp != nil {
		// If the subscription is Ready, then we have to remove it
		// from the channel's subscriber list.
		if subscription.Status.IsReady() {
			err := r.syncPhysicalChannel(subscription, true)
			if err != nil {
				glog.Warningf("Failed to sync physical from Channel : %s", err)
				r.recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
				return err
			}
		}
		removeFinalizer(subscription)
		return nil
	}

	// Verify that `channel` exists.
	_, err = r.fetchObjectReference(subscription.Namespace, &subscription.Spec.Channel)
	if err != nil {
		glog.Warningf("Failed to validate `channel` exists: %+v, %v", subscription.Spec.Channel, err)
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFetchFailed, "Failed to validate spec.channel exists: %v", err)
		return err
	}

	if subscriberURI, err := r.resolveSubscriberSpec(subscription.Namespace, subscription.Spec.Subscriber); err != nil {
		glog.Warningf("Failed to resolve Subscriber %+v : %s", *subscription.Spec.Subscriber, err)
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
		return err
	} else {
		subscription.Status.PhysicalSubscription.SubscriberURI = subscriberURI
		glog.Infof("Resolved subscriber to: %q", subscriberURI)
	}

	if replyURI, err := r.resolveResult(subscription.Namespace, subscription.Spec.Reply); err != nil {
		glog.Warningf("Failed to resolve Result %v : %v", subscription.Spec.Reply, err)
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, resultResolveFailed, "Failed to resolve spec.reply: %v", err)
		return err
	} else {
		subscription.Status.PhysicalSubscription.ReplyURI = replyURI
		glog.Infof("Resolved reply to: %q", replyURI)
	}

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	// Ok, now that we have the Channel and at least one of the Call/Result, let's reconcile
	// the Channel with this information.
	err = r.syncPhysicalChannel(subscription, false)
	if err != nil {
		glog.Warningf("Failed to sync physical Channel : %s", err)
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
		return err
	}
	// Everything went well, set the fact that subscriptions have been modified
	subscription.Status.MarkChannelReady()
	addFinalizer(subscription)
	return nil
}

func isNilOrEmptySubscriber(sub *v1alpha1.SubscriberSpec) bool {
	return sub == nil || equality.Semantic.DeepEqual(sub, &v1alpha1.SubscriberSpec{})
}

func isNilOrEmptyReply(reply *v1alpha1.ReplyStrategy) bool {
	return reply == nil || equality.Semantic.DeepEqual(reply, &v1alpha1.ReplyStrategy{})
}

// updateStatus may in fact update the subscription's finalizers in addition to the status
func (r *reconciler) updateStatus(subscription *v1alpha1.Subscription) (*v1alpha1.Subscription, error) {
	objectKey := client.ObjectKey{Namespace: subscription.Namespace, Name: subscription.Name}
	latestSubscription := &v1alpha1.Subscription{}

	if err := r.client.Get(context.TODO(), objectKey, latestSubscription); err != nil {
		return nil, err
	}

	subscriptionChanged := false

	if !equality.Semantic.DeepEqual(latestSubscription.Finalizers, subscription.Finalizers) {
		latestSubscription.SetFinalizers(subscription.ObjectMeta.Finalizers)
		if err := r.client.Update(context.TODO(), latestSubscription); err != nil {
			return nil, err
		}
		subscriptionChanged = true
	}

	if equality.Semantic.DeepEqual(latestSubscription.Status, subscription.Status) {
		return latestSubscription, nil
	}

	if subscriptionChanged {
		// Refetch
		latestSubscription = &v1alpha1.Subscription{}
		if err := r.client.Get(context.TODO(), objectKey, latestSubscription); err != nil {
			return nil, err
		}
	}

	latestSubscription.Status = subscription.Status
	if err := r.client.Status().Update(context.TODO(), latestSubscription); err != nil {
		return nil, err
	}

	return latestSubscription, nil
}

// resolveSubscriberSpec resolves the Spec.Call object. If it's an
// ObjectReference will resolve the object and treat it as a Callable. If
// it's DNSName then it's used as is.
// TODO: Once Service Routes, etc. support Callable, use that.
//
func (r *reconciler) resolveSubscriberSpec(namespace string, s *v1alpha1.SubscriberSpec) (string, error) {
	if isNilOrEmptySubscriber(s) {
		return "", nil
	}
	if s.DNSName != nil && *s.DNSName != "" {
		return *s.DNSName, nil
	}

	// K8s services are special cased. They can be called, even though they do not satisfy the
	// Callable interface.
	if s.Ref != nil && s.Ref.APIVersion == "v1" && s.Ref.Kind == "Service" {
		svc := &corev1.Service{}
		svcKey := types.NamespacedName{
			Namespace: namespace,
			Name:      s.Ref.Name,
		}
		err := r.client.Get(context.TODO(), svcKey, svc)
		if err != nil {
			glog.Warningf("Failed to fetch SubscriberSpec target as a K8s Service %+v: %s", s.Ref, err)
			return "", err
		}
		return domainToURL(names.ServiceHostName(svc.Name, svc.Namespace)), nil
	}

	obj, err := r.fetchObjectReference(namespace, s.Ref)
	if err != nil {
		glog.Warningf("Failed to fetch SubscriberSpec target %+v: %s", s.Ref, err)
		return "", err
	}
	t := duckv1alpha1.AddressableType{}
	if err := duck.FromUnstructured(obj, &t); err == nil {
		if t.Status.Address != nil {
			return domainToURL(t.Status.Address.Hostname), nil
		}
	}

	legacy := duckv1alpha1.LegacyTarget{}
	if err := duck.FromUnstructured(obj, &legacy); err == nil {
		if legacy.Status.DomainInternal != "" {
			return domainToURL(legacy.Status.DomainInternal), nil
		}
	}

	return "", fmt.Errorf("status does not contain address")
}

// resolveResult resolves the Spec.Result object.
func (r *reconciler) resolveResult(namespace string, replyStrategy *v1alpha1.ReplyStrategy) (string, error) {
	if isNilOrEmptyReply(replyStrategy) {
		return "", nil
	}
	obj, err := r.fetchObjectReference(namespace, replyStrategy.Channel)
	if err != nil {
		glog.Warningf("Failed to fetch ReplyStrategy channel %+v: %s", replyStrategy, err)
		return "", err
	}
	s := duckv1alpha1.AddressableType{}
	err = duck.FromUnstructured(obj, &s)
	if err != nil {
		glog.Warningf("Failed to deserialize Addressable target: %s", err)
		return "", err
	}
	if s.Status.Address != nil {
		return domainToURL(s.Status.Address.Hostname), nil
	}
	return "", fmt.Errorf("status does not contain address")
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

func (r *reconciler) syncPhysicalChannel(sub *v1alpha1.Subscription, isDeleted bool) error {
	glog.Infof("Reconciling Physical From Channel: %+v", sub)

	subs, err := r.listAllSubscriptionsWithPhysicalChannel(sub)
	if err != nil {
		glog.Infof("Unable to list all subscriptions with physical channel: %+v", err)
		return err
	}

	if !isDeleted {
		// The sub we are currently reconciling has not yet written any updated status, so when listing
		// it won't show any updates to the Status.PhysicalSubscription. We know that we are listing
		// for subscriptions with the same PhysicalSubscription.From, so just add this one manually.
		subs = append(subs, *sub)
	}
	subscribable := r.createSubscribable(subs)

	if patchErr := r.patchPhysicalFrom(sub.Namespace, sub.Spec.Channel, subscribable); patchErr != nil {
		if isDeleted && errors.IsNotFound(patchErr) {
			glog.Infof("could not find channel %v\n", sub.Spec.Channel)
			return nil
		}
		return patchErr
	}
	return nil
}

func (r *reconciler) listAllSubscriptionsWithPhysicalChannel(sub *v1alpha1.Subscription) ([]v1alpha1.Subscription, error) {
	subs := make([]v1alpha1.Subscription, 0)

	opts := &client.ListOptions{
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "Subscription",
			},
		},
		Namespace: sub.Namespace,
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
				// This is the sub that is being reconciled. Skip it.
				continue
			}
			if equality.Semantic.DeepEqual(sub.Spec.Channel, s.Spec.Channel) {
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

func (r *reconciler) createSubscribable(subs []v1alpha1.Subscription) *eventingduck.Subscribable {
	rv := &eventingduck.Subscribable{}
	for _, sub := range subs {
		if sub.Status.PhysicalSubscription.SubscriberURI != "" || sub.Status.PhysicalSubscription.ReplyURI != "" {
			rv.Subscribers = append(rv.Subscribers, eventingduck.ChannelSubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: sub.APIVersion,
					Kind:       sub.Kind,
					Namespace:  sub.Namespace,
					Name:       sub.Name,
					UID:        sub.UID,
				},
				SubscriberURI: sub.Status.PhysicalSubscription.SubscriberURI,
				ReplyURI:      sub.Status.PhysicalSubscription.ReplyURI,
			})
		}
	}
	return rv
}

func (r *reconciler) patchPhysicalFrom(namespace string, physicalFrom corev1.ObjectReference, subs *eventingduck.Subscribable) error {
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
	after.Spec.Subscribable = subs

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

func addFinalizer(sub *v1alpha1.Subscription) {
	finalizers := sets.NewString(sub.Finalizers...)
	finalizers.Insert(finalizerName)
	sub.Finalizers = finalizers.List()
}

func removeFinalizer(sub *v1alpha1.Subscription) {
	finalizers := sets.NewString(sub.Finalizers...)
	finalizers.Delete(finalizerName)
	sub.Finalizers = finalizers.List()
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}
