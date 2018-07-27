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

package flow

import (
	"context"
	"fmt"
	"log"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	v1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TODO: This should come from a configmap
const defaultBusName = "stub"

// What field do we assume Object Reference exports as a resolvable target
const targetFieldName = "domainInternal"

var (
	flowControllerKind = v1alpha1.SchemeGroupVersion.WithKind("Flow")
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Flow resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Reconciling flow %v", request)
	flow := &v1alpha1.Flow{}
	err := r.client.Get(context.TODO(), request.NamespacedName, flow)

	if errors.IsNotFound(err) {
		glog.Errorf("could not find flow %v\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		glog.Errorf("could not fetch Flow %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	original := flow.DeepCopy()

	flow.Status.InitializeConditions()

	// Reconcile this copy of the Flow and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(flow)
	if equality.Semantic.DeepEqual(original.Status, flow.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := r.updateStatus(flow); err != nil {
		glog.Warningf("Failed to update flow status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(flow *v1alpha1.Flow) error {
	// See if the flow has been deleted
	accessor, err := meta.Accessor(flow)
	if err != nil {
		log.Fatalf("Failed to get metadata: %s", err)
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	glog.Infof("DeletionTimestamp: %v", deletionTimestamp)

	target, err := r.resolveActionTarget(flow.Namespace, flow.Spec.Action)
	if err != nil {
		glog.Warningf("Failed to resolve target %v : %v", flow.Spec.Action, err)
		flow.Status.PropagateActionTargetResolved(corev1.ConditionFalse, "ActionTargetNotResolved", err.Error())
		return err
	}

	flow.Status.PropagateActionTargetResolved(corev1.ConditionTrue, "ActionTargetResolved", fmt.Sprintf("Resolved to: %q", target))

	// Ok, so target is the underlying k8s service (or URI if so specified) that we want to target
	glog.Infof("Resolved Target to: %q", target)

	// Reconcile the Channel. Creates a channel that is the target that the Feed will use.
	// TODO: We should reuse channels possibly. By this I mean that instead of creating a
	// channel for each subdscription, we could look at existing channels and reuse one
	// and only create a subscription to a channel instead.
	channel, err := r.reconcileChannel(flow)
	if err != nil {
		glog.Warningf("Failed to reconcile channel : %v", err)
		return err
	}
	flow.Status.PropagateChannelStatus(channel.Status)

	subscription, err := r.reconcileSubscription(channel.Name, target, flow)
	if err != nil {
		glog.Warningf("Failed to reconcile subscription : %v", err)
		return err
	}
	flow.Status.PropagateSubscriptionStatus(subscription.Status)

	channelDNS := channel.Status.DomainInternal
	if channelDNS != "" {
		glog.Infof("Reconciling feed for flow %q targeting %q", flow.Name, channelDNS)
		feed, err := r.reconcileFeed(channelDNS, flow)
		if err != nil {
			glog.Warningf("Failed to reconcile feed: %v", err)
			return err
		}
		glog.Infof("Reconciled feed status for flow %q targeting %q : %+v", flow.Name, channelDNS, feed.Status)
		flow.Status.PropagateFeedStatus(feed.Status)
	}
	return nil
}

func (r *reconciler) updateStatus(flow *v1alpha1.Flow) (*v1alpha1.Flow, error) {
	newFlow := &v1alpha1.Flow{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: flow.Namespace, Name: flow.Name}, newFlow)

	if err != nil {
		return nil, err
	}
	newFlow.Status = flow.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Flow resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err = r.client.Update(context.TODO(), newFlow); err != nil {
		return nil, err
	}
	return newFlow, nil
}

// resolveActionTarget resolves the Action.Target. If it's an ObjectReference
// will resolve it, and if it's an TargetURI will just return it.
func (r *reconciler) resolveActionTarget(namespace string, action v1alpha1.FlowAction) (string, error) {
	glog.Infof("Resolving target: %+v", action)

	if action.Target != nil {
		return r.resolveObjectReference(namespace, action.Target)
	}
	if action.TargetURI != nil {
		return *action.TargetURI, nil
	}

	return "", fmt.Errorf("No resolvable action target: %+v", action)
}

// resolveObjectReference fetches an object based on ObjectReference. It assumes the
// object has a status["domainInternal"] string in it and returns it.
func (r *reconciler) resolveObjectReference(namespace string, ref *corev1.ObjectReference) (string, error) {
	resourceClient, err := CreateResourceInterface(r.restConfig, ref, namespace)
	if err != nil {
		glog.Warningf("failed to create dynamic client resource: %v", err)
		return "", err
	}

	obj, err := resourceClient.Get(ref.Name, metav1.GetOptions{})
	if err != nil {
		glog.Warningf("failed to get object: %v", err)
		return "", err
	}
	status, ok := obj.Object["status"]
	if !ok {
		return "", fmt.Errorf("%q does not contain status", ref.Name)
	}
	statusMap := status.(map[string]interface{})
	serviceName, ok := statusMap[targetFieldName]
	if !ok {
		return "", fmt.Errorf("%q does not contain field %q in status", targetFieldName, ref.Name)
	}
	serviceNameStr, ok := serviceName.(string)
	if !ok {
		return "", fmt.Errorf("%q status field %q is not a string", targetFieldName, ref.Name)
	}
	return serviceNameStr, nil
}

func (r *reconciler) reconcileChannel(flow *v1alpha1.Flow) (*channelsv1alpha1.Channel, error) {
	channelName := flow.Name

	channel := &channelsv1alpha1.Channel{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: flow.Namespace, Name: channelName}, channel)
	if errors.IsNotFound(err) {
		channel, err = r.createChannel(flow)
		if err != nil {
			glog.Errorf("Failed to create channel %q : %v", channelName, err)
			return nil, err
		}
	} else if err != nil {
		glog.Errorf("Failed to reconcile channel %q failed to get channels : %v", channelName, err)
		return nil, err
	}

	// TODO: Make sure channel is what it should be. For now, just assume it's fine
	// if it exists.
	return channel, nil
}

func (r *reconciler) createChannel(flow *v1alpha1.Flow) (*channelsv1alpha1.Channel, error) {
	channelName := flow.Name
	channel := &channelsv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      channelName,
			Namespace: flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*r.NewControllerRef(flow),
			},
		},
		Spec: channelsv1alpha1.ChannelSpec{
			ClusterBus: defaultBusName,
		},
	}
	if err := r.client.Create(context.TODO(), channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (r *reconciler) reconcileSubscription(channelName string, target string, flow *v1alpha1.Flow) (*channelsv1alpha1.Subscription, error) {
	subscriptionName := flow.Name

	subscription := &channelsv1alpha1.Subscription{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: flow.Namespace, Name: subscriptionName}, subscription)
	if errors.IsNotFound(err) {
		subscription, err = r.createSubscription(channelName, target, flow)
		if err != nil {
			glog.Errorf("Failed to create subscription %q : %v", subscriptionName, err)
			return nil, err
		}
	} else if err != nil {
		glog.Errorf("Failed to reconcile subscription %q failed to get subscriptions : %v", subscriptionName, err)
		return nil, err
	}

	// TODO: Make sure subscription is what it should be. For now, just assume it's fine
	// if it exists.
	return subscription, nil
}

func (r *reconciler) createSubscription(channelName string, target string, flow *v1alpha1.Flow) (*channelsv1alpha1.Subscription, error) {
	subscriptionName := flow.Name
	subscription := &channelsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      subscriptionName,
			Namespace: flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*r.NewControllerRef(flow),
			},
		},
		Spec: channelsv1alpha1.SubscriptionSpec{
			Channel:    channelName,
			Subscriber: target,
		},
	}
	if err := r.client.Create(context.TODO(), subscription); err != nil {
		return nil, err
	}
	return subscription, nil
}

func (r *reconciler) reconcileFeed(channelDNS string, flow *v1alpha1.Flow) (*feedsv1alpha1.Feed, error) {
	feedName := flow.Name

	feed := &feedsv1alpha1.Feed{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: flow.Namespace, Name: feedName}, feed)
	if errors.IsNotFound(err) {
		feed, err = r.createFeed(channelDNS, flow)
		if err != nil {
			glog.Errorf("Failed to create feed %q : %v", feedName, err)
			return nil, err
		}
	} else if err != nil {
		glog.Errorf("Failed to reconcile feed %q failed to get feeds : %v", feedName, err)
		return nil, err
	}

	glog.Infof("Reconciled feed: %+v", feed)
	// TODO: Make sure feed is what it should be. For now, just assume it's fine
	// if it exists.
	return feed, nil

}

func (r *reconciler) createFeed(channelDNS string, flow *v1alpha1.Flow) (*feedsv1alpha1.Feed, error) {
	feedName := flow.Name
	feed := &feedsv1alpha1.Feed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      feedName,
			Namespace: flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*r.NewControllerRef(flow),
			},
		},
		Spec: feedsv1alpha1.FeedSpec{
			Action: feedsv1alpha1.FeedAction{DNSName: channelDNS},
			Trigger: feedsv1alpha1.EventTrigger{
				EventType: flow.Spec.Trigger.EventType,
				Resource:  flow.Spec.Trigger.Resource,
				Service:   flow.Spec.Trigger.Service,
			},
		},
	}
	if flow.Spec.ServiceAccountName != "" {
		feed.Spec.ServiceAccountName = flow.Spec.ServiceAccountName
	}

	if flow.Spec.Trigger.Parameters != nil {
		feed.Spec.Trigger.Parameters = flow.Spec.Trigger.Parameters
	}
	if flow.Spec.Trigger.ParametersFrom != nil {
		feed.Spec.Trigger.ParametersFrom = flow.Spec.Trigger.ParametersFrom
	}

	if err := r.client.Create(context.TODO(), feed); err != nil {
		return nil, err
	}
	return feed, nil
}

func (r *reconciler) NewControllerRef(flow *v1alpha1.Flow) *metav1.OwnerReference {
	blockOwnerDeletion := false
	isController := true
	revRef := metav1.NewControllerRef(flow, flowControllerKind)
	revRef.BlockOwnerDeletion = &blockOwnerDeletion
	revRef.Controller = &isController
	return revRef
}
