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
	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingduck "github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler/channel/resources"
	"github.com/knative/eventing/pkg/utils"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/apis/duck"
	duckapis "knative.dev/pkg/apis/duck"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	duckroot "knative.dev/pkg/apis"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"go.uber.org/zap"
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
	channelLister   listers.ChannelLister
	resourceTracker eventingduck.ResourceTracker
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

	channelResourceInterface := r.DynamicClientSet.Resource(duckroot.KindToResource(c.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())).Namespace(c.Namespace)

	// Tell tracker to reconcile this Channel whenever the backing Channel CRD changes.
	track := r.resourceTracker.TrackInNamespace(c)

	backingChannel, err := r.reconcileBackingChannel(ctx, channelResourceInterface, c)
	if err != nil {
		c.Status.MarkBackingChannelFailed("ChannelFailure", "%v", err)
		return fmt.Errorf("problem reconciling the backing channel: %v", err)
	}

	// Start tracking the backing Channel CRD...
	if err = track(utils.ObjectRef(backingChannel, backingChannel.GroupVersionKind())); err != nil {
		return fmt.Errorf("unable to track changes to the backing channel: %v", err)
	}

	c.Status.Channel = &corev1.ObjectReference{
		Kind:       backingChannel.Kind,
		APIVersion: backingChannel.APIVersion,
		Name:       backingChannel.Name,
		Namespace:  backingChannel.Namespace,
	}

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
func (r *Reconciler) reconcileBackingChannel(ctx context.Context, resourceClient dynamic.ResourceInterface, c *v1alpha1.Channel) (*duckv1alpha1.Channelable, error) {
	channel, err := resourceClient.Get(c.Name, metav1.GetOptions{})
	channelable := &duckv1alpha1.Channelable{}

	// If the resource doesn't exist, we'll create it
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := resources.NewChannel(c)
			if err != nil {
				logging.FromContext(ctx).Error("Failed to create Channel from ChannelTemplate", zap.Any("channelTemplate", c.Spec.ChannelTemplate), zap.Error(err))
				return nil, err
			}
			channel, err = resourceClient.Create(newChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error("Failed to create Channel", zap.Any("channel", newChannel), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Info("Created Channel", zap.Any("channel", newChannel))
		} else {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to get Channel: %s/%s", c.Namespace, c.Name), zap.Error(err))
			return nil, err
		}
	}
	logging.FromContext(ctx).Debug("Found Channel", zap.Any("channel", c))
	err = duckapis.FromUnstructured(channel, channelable)
	if err != nil {
		logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Error(err), zap.Any("channel", channel))
		return nil, err
	}
	return channelable, nil
}

func (r *Reconciler) patchBackingChannelSubscriptions(ctx context.Context, resourceClient dynamic.ResourceInterface, channel *v1alpha1.Channel, backingChannel *duckv1alpha1.Channelable) error {
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

	patched, err := resourceClient.Patch(backingChannel.GetName(), types.MergePatchType, patch, metav1.UpdateOptions{})
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to patch the Channel", zap.Error(err), zap.Any("patch", patch))
		return err
	}
	logging.FromContext(ctx).Debug("Patched resource", zap.Any("patched", patched))
	return nil
}
