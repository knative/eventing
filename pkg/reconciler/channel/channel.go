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
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	channelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/channel"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	eventingduck "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/channel/resources"
	duckapis "knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
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
	bCS := r.getChannelableStatus(ctx, &backingChannel.Status, backingChannel.Annotations)
	c.Status.PropagateStatuses(bCS)

	return nil
}

func (r *Reconciler) getChannelableStatus(ctx context.Context, bc *duckv1alpha1.ChannelableCombinedStatus, cAnnotations map[string]string) *eventingduckv1.ChannelableStatus {

	channelableStatus := &eventingduckv1.ChannelableStatus{}
	if bc.AddressStatus.Address != nil {
		channelableStatus.AddressStatus.Address = &duckv1.Addressable{}
		bc.AddressStatus.Address.ConvertTo(ctx, channelableStatus.AddressStatus.Address)
	}
	channelableStatus.Status = bc.Status
	if cAnnotations != nil &&
		cAnnotations[messaging.SubscribableDuckVersionAnnotation] == "v1" || cAnnotations[messaging.SubscribableDuckVersionAnnotation] == "v1beta1" {
		subs := make([]eventingduckv1.SubscriberStatus, len(bc.SubscribableStatus.Subscribers))
		for i, sub := range bc.SubscribableStatus.Subscribers {
			sub.ConvertTo(ctx, &subs[i])
		}
		if len(bc.SubscribableStatus.Subscribers) > 0 {
			channelableStatus.SubscribableStatus.Subscribers = subs
		}
	} else { //we assume v1alpha1 if no tag according to the spec
		if bc.SubscribableTypeStatus.SubscribableStatus != nil &&
			len(bc.SubscribableTypeStatus.SubscribableStatus.Subscribers) > 0 {
			channelableStatus.SubscribableStatus.Subscribers = make([]eventingduckv1.SubscriberStatus, len(bc.SubscribableTypeStatus.SubscribableStatus.Subscribers))
			for i, ss := range bc.SubscribableTypeStatus.SubscribableStatus.Subscribers {
				channelableStatus.SubscribableStatus.Subscribers[i] = eventingduckv1.SubscriberStatus{
					UID:                ss.UID,
					ObservedGeneration: ss.ObservedGeneration,
					Ready:              ss.Ready,
					Message:            ss.Message,
				}
			}
		}
	}
	return channelableStatus
}

// reconcileBackingChannel reconciles Channel's 'c' underlying CRD channel.
func (r *Reconciler) reconcileBackingChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, c *v1.Channel, backingChannelObjRef duckv1.KReference) (*duckv1alpha1.ChannelableCombined, error) {
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
			newBackingChannel, err := resources.NewChannel(c)
			if err != nil {
				logger.Errorw("Failed to create Channel from ChannelTemplate", zap.Any("channelTemplate", c.Spec.ChannelTemplate), zap.Error(err))
				return nil, err
			}
			logger.Debugf("Creating Channel Object: %+v", newBackingChannel)
			created, err := channelResourceInterface.Create(newBackingChannel, metav1.CreateOptions{})
			if err != nil {
				logger.Errorw("Failed to create backing Channel", zap.Any("backingChannel", newBackingChannel), zap.Error(err))
				return nil, err
			}
			logger.Debug("Created backing Channel", zap.Any("backingChannel", newBackingChannel))
			channelable := &duckv1alpha1.ChannelableCombined{}
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
	channelable, ok := backingChannel.(*duckv1alpha1.ChannelableCombined)
	if !ok {
		return nil, fmt.Errorf("Failed to convert to Channelable Object %+v", backingChannel)
	}
	return channelable, nil
}
