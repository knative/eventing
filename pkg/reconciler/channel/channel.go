/*
Copyright 2019 The Knative Authors

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

package channel

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging/v1beta1"
	channelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1beta1/channel"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1beta1"
	eventingduck "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/channel/resources"
	duckapis "knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason ChannelReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: \"%s/%s\"", namespace, name)
}

type Reconciler struct {
	// listers index properties about resources
	channelLister      listers.ChannelLister
	channelableTracker eventingduck.ListableTracker

	// dynamicClientSet allows us to configure pluggable Build objects
	dynamicClientSet dynamic.Interface
}

// Check that our Reconciler implements Interface
var _ channelreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, c *v1beta1.Channel) pkgreconciler.Event {
	c.Status.InitializeConditions()
	c.Status.ObservedGeneration = c.Generation

	// 1. Create the backing Channel CRD, if it doesn't exist.
	// 2. Propagate the backing Channel CRD Status, Address, and SubscribableStatus into this Channel.

	gvr, _ := meta.UnsafeGuessKindToResource(c.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.dynamicClientSet.Resource(gvr).Namespace(c.Namespace)
	if channelResourceInterface == nil {
		return fmt.Errorf("unable to create dynamic client for: %+v", c.Spec.ChannelTemplate)
	}

	track := r.channelableTracker.TrackInNamespaceKReference(c)

	backingChannelObjRef := duckv1.KReference{
		Kind:       c.Spec.ChannelTemplate.Kind,
		APIVersion: c.Spec.ChannelTemplate.APIVersion,
		Name:       c.Name,
		Namespace:  c.Namespace,
	}
	// Tell the channelTracker to reconcile this Channel whenever the backing Channel changes.
	if err := track(backingChannelObjRef); err != nil {
		return fmt.Errorf("unable to track changes to the backing Channel: %v", err)
	}

	backingChannel, err := r.reconcileBackingChannel(ctx, channelResourceInterface, c, backingChannelObjRef)
	if err != nil {
		c.Status.MarkBackingChannelFailed("ChannelFailure", "%v", err)
		return fmt.Errorf("problem reconciling the backing channel: %v", err)
	}

	c.Status.Channel = &backingChannelObjRef
	c.Status.PropagateStatuses(&backingChannel.Status)

	return newReconciledNormal(c.Namespace, c.Name)
}

// reconcileBackingChannel reconciles Channel's 'c' underlying CRD channel.
func (r *Reconciler) reconcileBackingChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, c *v1beta1.Channel, backingChannelObjRef duckv1.KReference) (*duckv1beta1.Channelable, error) {
	lister, err := r.channelableTracker.ListerForKReference(backingChannelObjRef)
	if err != nil {
		logging.FromContext(ctx).Error("Error getting lister for Channel", zap.Any("backingChannel", backingChannelObjRef), zap.Error(err))
		return nil, err
	}
	backingChannel, err := lister.ByNamespace(backingChannelObjRef.Namespace).Get(backingChannelObjRef.Name)
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if apierrs.IsNotFound(err) {
			newBackingChannel, err := resources.NewChannel(c)
			if err != nil {
				logging.FromContext(ctx).Error("Failed to create Channel from ChannelTemplate", zap.Any("channelTemplate", c.Spec.ChannelTemplate), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Debug(fmt.Sprintf("Creating Channel Object: %+v", newBackingChannel))
			created, err := channelResourceInterface.Create(newBackingChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error("Failed to create backing Channel", zap.Any("backingChannel", newBackingChannel), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Debug("Created backing Channel", zap.Any("backingChannel", newBackingChannel))
			channelable := &duckv1beta1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Any("backingChannel", backingChannelObjRef), zap.Any("createdChannel", created), zap.Error(err))
				return nil, err

			}
			return channelable, nil
		}
		logging.FromContext(ctx).Info("Failed to get backing Channel", zap.Any("backingChannel", backingChannelObjRef), zap.Error(err))
		return nil, err
	}
	logging.FromContext(ctx).Debug("Found backing Channel", zap.Any("backingChannel", backingChannelObjRef))
	channelable, ok := backingChannel.(*duckv1beta1.Channelable)
	if !ok {
		logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Any("backingChannel", backingChannel), zap.Error(err))
		return nil, err
	}
	return channelable, nil
}
