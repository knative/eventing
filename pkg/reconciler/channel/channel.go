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
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduck "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/channel/resources"
	"knative.dev/pkg/apis/duck"
	duckapis "knative.dev/pkg/apis/duck"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/controller"
)

const (
	channelReadinessChanged   = "ChannelReadinessChanged"
	channelReconciled         = "ChannelReconciled"
	channelReconcileError     = "ChannelReconcileError"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	channelLister      listers.ChannelLister
	channelableTracker eventingduck.ListableTracker
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the Channel resource with this namespace/name
	original, err := r.channelLister.Channels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("Channel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Delete is a no-op.
	if original.DeletionTimestamp != nil {
		return nil
	}

	// Don't modify the informers copy
	channel := original.DeepCopy()

	// Reconcile this copy of the Channel and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, channel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Channel", zap.Error(reconcileErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelReconcileError, fmt.Sprintf("Channel reconcile error: %v", reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("Successfully reconciled Channel")
		r.Recorder.Eventf(channel, corev1.EventTypeNormal, channelReconciled, "Channel reconciled: %s", key)
	}

	if _, updateStatusErr := r.updateStatus(ctx, channel.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Error updating Channel status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update channel status: %s", key)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, c *v1alpha1.Channel) error {
	c.Status.InitializeConditions()

	// 1. Create the backing Channel CRD, if it doesn't exist.
	// 2. Patch the subscriptions from this Channel into the backing Channel CRD.
	// 3. Propagate the backing Channel CRD Status, Address, and SubscribableStatus into this Channel.

	if c.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	gvr, _ := meta.UnsafeGuessKindToResource(c.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.DynamicClientSet.Resource(gvr).Namespace(c.Namespace)
	if channelResourceInterface == nil {
		return fmt.Errorf("unable to create dynamic client for: %+v", c.Spec.ChannelTemplate)
	}

	track := r.channelableTracker.TrackInNamespace(c)

	backingChannelObjRef := corev1.ObjectReference{
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

	err = r.patchBackingChannelSubscriptions(ctx, channelResourceInterface, c, backingChannel)
	if err != nil {
		c.Status.MarkBackingChannelFailed("ChannelFailure", "%v", err)
		return fmt.Errorf("problem patching subscriptions in the backing channel: %v", err)
	}

	c.Status.PropagateStatuses(&backingChannel.Status)
	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Channel) (*v1alpha1.Channel, error) {
	channel, err := r.channelLister.Channels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(channel.Status, desired.Status) {
		return channel, nil
	}

	becomesReady := desired.Status.IsReady() && !channel.Status.IsReady()

	// Don't modify the informers copy.
	existing := channel.DeepCopy()
	existing.Status = desired.Status

	c, err := r.EventingClientSet.MessagingV1alpha1().Channels(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		duration := time.Since(c.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Sugar().Infof("Channel %q became ready after %v", channel.Name, duration)
		r.Recorder.Event(channel, corev1.EventTypeNormal, channelReadinessChanged, fmt.Sprintf("Channel %q became ready", channel.Name))
		if err := r.StatsReporter.ReportReady("Channel", channel.Namespace, channel.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for Channel, %v", err)
		}
	}

	return c, err
}

// reconcileBackingChannel reconciles Channel's 'c' underlying CRD channel.
func (r *Reconciler) reconcileBackingChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, c *v1alpha1.Channel, backingChannelObjRef corev1.ObjectReference) (*duckv1alpha1.Channelable, error) {
	lister, err := r.channelableTracker.ListerFor(backingChannelObjRef)
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
			channelable := &duckv1alpha1.Channelable{}
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
	channelable, ok := backingChannel.(*duckv1alpha1.Channelable)
	if !ok {
		logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Any("backingChannel", backingChannel), zap.Error(err))
		return nil, err
	}
	return channelable, nil
}

func (r *Reconciler) patchBackingChannelSubscriptions(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, channel *v1alpha1.Channel, backingChannel *duckv1alpha1.Channelable) error {
	if equality.Semantic.DeepEqual(channel.Spec.Subscribable, backingChannel.Spec.Subscribable) {
		logging.FromContext(ctx).Debug("Subscribable in sync, no need to patch")
		return nil
	}

	after := backingChannel.DeepCopy()
	after.Spec.Subscribable = channel.Spec.Subscribable

	patch, err := duck.CreateMergePatch(backingChannel, after)

	if err != nil {
		logging.FromContext(ctx).Warn("Failed to create mergePatch", zap.Error(err))
		return err
	}
	patched, err := channelResourceInterface.Patch(backingChannel.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to patch the Channel", zap.Error(err), zap.Any("patch", patch))
		return err
	}
	logging.FromContext(ctx).Debug("Patched resource", zap.Any("patched", patched))
	return nil
}
