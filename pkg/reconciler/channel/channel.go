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
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/kmeta"

	duckapis "knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	channelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/channel"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	ducklib "knative.dev/eventing/pkg/duck"
	eventingduck "knative.dev/eventing/pkg/duck"
)

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
func (r *Reconciler) ReconcileKind(ctx context.Context, c *v1.Channel) pkgreconciler.Event {
	// 1. Create the backing Channel CRD, if it doesn't exist.
	// 2. Propagate the backing Channel CRD Status, Address, and SubscribableStatus into this Channel.

	gvr, _ := meta.UnsafeGuessKindToResource(c.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.dynamicClientSet.Resource(gvr).Namespace(c.Namespace)
	if channelResourceInterface == nil {
		return fmt.Errorf("unable to create dynamic client for: %+v", c.Spec.ChannelTemplate)
	}

	track := r.channelableTracker.TrackInNamespaceKReference(ctx, c)

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

	// If a DeadLetterSink is defined in Spec.Delivery then whe resolve its URI and update the stauts
	if c.Spec.Delivery != nil && c.Spec.Delivery.DeadLetterSink != nil {
		if backingChannel.Status.DeliveryStatus.IsSet() {
			c.Status.MarkDeadLetterSinkResolvedSucceeded(backingChannel.Status.DeliveryStatus)
		} else {
			c.Status.MarkDeadLetterSinkResolvedFailed(fmt.Sprintf("Backing Channel %s didn't set status.deadLetterSinkURI", backingChannel.Name), "")
		}
	} else {
		c.Status.MarkDeadLetterSinkNotConfigured()
	}

	return nil
}

// reconcileBackingChannel reconciles Channel's 'c' underlying CRD channel.
func (r *Reconciler) reconcileBackingChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, c *v1.Channel, backingChannelObjRef duckv1.KReference) (*eventingduckv1.Channelable, error) {
	logger := logging.FromContext(ctx)
	lister, err := r.channelableTracker.ListerForKReference(backingChannelObjRef)
	if err != nil {
		logger.Errorw("Error getting lister for Channel", zap.Any("backingChannel", backingChannelObjRef), zap.Error(err))
		return nil, err
	}
	backingChannel, err := lister.ByNamespace(backingChannelObjRef.Namespace).Get(backingChannelObjRef.Name)
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if apierrs.IsNotFound(err) {
			newBackingChannel, err := ducklib.NewPhysicalChannel(
				c.Spec.ChannelTemplate.TypeMeta,
				metav1.ObjectMeta{
					Name:      c.Name,
					Namespace: c.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*kmeta.NewControllerRef(c),
					},
				},
				ducklib.WithChannelableSpec(c.Spec.ChannelableSpec),
				ducklib.WithPhysicalChannelSpec(c.Spec.ChannelTemplate.Spec),
			)
			if err != nil {
				logger.Errorw("Failed to create Channel from ChannelTemplate", zap.Any("channelTemplate", c.Spec.ChannelTemplate), zap.Error(err))
				return nil, err
			}
			logger.Debugf("Creating Channel Object: %+v", newBackingChannel)
			created, err := channelResourceInterface.Create(ctx, newBackingChannel, metav1.CreateOptions{})
			if err != nil {
				logger.Errorw("Failed to create backing Channel", zap.Any("backingChannel", newBackingChannel), zap.Error(err))
				return nil, err
			}
			logger.Debug("Created backing Channel", zap.Any("backingChannel", newBackingChannel))
			channelable := &eventingduckv1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logger.Errorw("Failed to convert to Channelable Object", zap.Any("backingChannel", backingChannelObjRef), zap.Any("createdChannel", created), zap.Error(err))
				return nil, err

			}
			return channelable, nil
		}
		logger.Infow("Failed to get backing Channel", zap.Any("backingChannel", backingChannelObjRef), zap.Error(err))
		return nil, err
	}
	logger.Debugw("Found backing Channel", zap.Any("backingChannel", backingChannelObjRef))
	channelable, ok := backingChannel.(*eventingduckv1.Channelable)
	if !ok {
		return nil, fmt.Errorf("Failed to convert to Channelable Object %+v", backingChannel)
	}
	return channelable, nil
}
