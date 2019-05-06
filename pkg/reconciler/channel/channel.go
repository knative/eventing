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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Channels"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName       = "channel-default-controller"
	channelReconciled         = "ChannelReconciled"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	channelLister listers.ChannelLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	channelInformer eventinginformers.ChannelInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:          reconciler.NewBase(opt, controllerAgentName),
		channelLister: channelInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.Logger.Info("Setting up event handlers")
	channelInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

// Reconcile will check if the channel is being watched by provisioner's channel controller
// This will improve UX. See https://github.com/knative/eventing/issues/779
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

func (r *Reconciler) reconcile(ctx context.Context, ch *v1alpha1.Channel) error {
	// Do not Initialize() Status in channel-default-controller. It will set ChannelConditionProvisionerInstalled=True
	// Directly call GetCondition(). If the Status was never initialized then GetCondition() will return nil and
	// IsUnknown() will return true
	c := ch.Status.GetCondition(v1alpha1.ChannelConditionProvisionerInstalled)

	if c == nil || c.IsUnknown() {

		var proName string
		var proKind string
		if ch.Spec.Provisioner != nil {
			proName = ch.Spec.Provisioner.Name
			proKind = ch.Spec.Provisioner.Kind
		}

		ch.Status.MarkProvisionerNotInstalled(
			"Provisioner not found.",
			"Specified provisioner [Name:%s Kind:%s] is not installed or not controlling the channel.",
			proName,
			proKind,
		)
	}
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

	// Don't modify the informers copy.
	existing := channel.DeepCopy()
	existing.Status = desired.Status

	return r.EventingClientSet.EventingV1alpha1().Channels(desired.Namespace).UpdateStatus(existing)
}
