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
	"errors"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	duckapis "knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
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
		if backingChannel.Status.DeliveryStatus.DeadLetterSinkURI != nil {
			c.Status.MarkDeadLetterSinkResolvedSucceeded(backingChannel.Status.DeliveryStatus.DeadLetterSinkURI)
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
	switch {
	case apierrs.IsNotFound(err):
		// Create the Channel if it doesn't exists.
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
			return nil, fmt.Errorf("failed to create physical channel from ChannelTemplate: %w", err)
		}
		logger.Infow("Creating Channel Object", zap.Any("Channel", newBackingChannel))
		created, err := channelResourceInterface.Create(ctx, newBackingChannel, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create physical channel: %w", err)
		}

		channelable := &eventingduckv1.Channelable{}
		err = duckapis.FromUnstructured(created, channelable)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Channelable to physical Channel: %w", err)
		}
		return channelable, nil

	case err != nil:
		return nil, fmt.Errorf("failed to get backing Channel: %w", err)
	}

	logger.Debugw("Found backing Channel", zap.Any("backingChannel", backingChannelObjRef))
	channelable, ok := backingChannel.(*eventingduckv1.Channelable)
	if !ok {
		return nil, fmt.Errorf("failed to convert to Channelable Object %+v", backingChannel)
	}

	// Make sure the existing physical Channel options match those of
	// the Channel, if not Patch.

	if equality.Semantic.DeepEqual(c.Spec.Delivery, channelable.Spec.Delivery) {
		// If propagated/mutable properties match return the Channel.
		return channelable, nil
	}

	// Create a Patch by isolating desired and existing Channel mutable
	// properties.
	jsonPatch, err := duckapis.CreatePatch(
		// Existing physical Channel properties
		eventingduckv1.Channelable{
			Spec: eventingduckv1.ChannelableSpec{
				Delivery: channelable.Spec.Delivery,
			},
		},
		// Channel (abstract) properties
		eventingduckv1.Channelable{
			Spec: eventingduckv1.ChannelableSpec{
				Delivery: c.Spec.Delivery,
			},
		})
	if err != nil {
		return nil, fmt.Errorf("creating JSON patch for Channelable Object: %w", err)
	}
	if len(jsonPatch) == 0 {
		// This is a very unexpected path.
		return nil, errors.New("unexpected empty JSON patch to update physical Channel")
	}

	patch, err := jsonPatch.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON patch for Channelable Object: %w", err)
	}

	logger.Info("Patching Channel", zap.String("patch", string(patch)))
	_, err = channelResourceInterface.Patch(ctx, backingChannelObjRef.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching channelable object: %w", err)
	}

	return channelable, nil
}
