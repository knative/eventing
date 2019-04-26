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

package eventtype

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/pkg/tracker"

	"k8s.io/client-go/tools/cache"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "EventTypes"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "eventtype-controller"

	// Name of the corev1.Events emitted from the reconciliation process.
	eventTypeReadinessChanged   = "EventTypeReadinessChanged"
	eventTypeReconcileFailed    = "EventTypeReconcileFailed"
	eventTypeUpdateStatusFailed = "EventTypeUpdateStatusFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	eventTypeLister listers.EventTypeLister
	brokerLister    listers.BrokerLister
	tracker         tracker.Interface
}

var brokerGVK = v1alpha1.SchemeGroupVersion.WithKind("Broker")

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	eventTypeInformer eventinginformers.EventTypeInformer,
	brokerInformer eventinginformers.BrokerInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:            reconciler.NewBase(opt, controllerAgentName),
		eventTypeLister: eventTypeInformer.Lister(),
		brokerLister:    brokerInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.Logger.Info("Setting up event handlers")
	eventTypeInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	// Tracker is used to notify us that a EventType's Broker has changed so that
	// we can reconcile.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	brokerInformer.Informer().AddEventHandler(reconciler.Handler(
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			v1alpha1.SchemeGroupVersion.WithKind("Broker"),
		),
	))

	return impl
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the EventType resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	ctx = logging.WithLogger(ctx, r.Logger.Desugar().With(zap.String("key", key)))

	// Get the EventType resource with this namespace/name
	original, err := r.eventTypeLister.EventTypes(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Info("eventType key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	eventType := original.DeepCopy()

	// Reconcile this copy of the EventType and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, eventType)
	if reconcileErr != nil {
		logging.FromContext(ctx).Warn("Error reconciling Broker", zap.Error(err))
		r.Recorder.Eventf(eventType, corev1.EventTypeWarning, eventTypeReconcileFailed, fmt.Sprintf("EventType reconcile error: %v", reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("EventType reconciled")
	}

	if _, err = r.updateStatus(ctx, eventType); err != nil {
		logging.FromContext(ctx).Warn("Failed to update the EventType status", zap.Error(err))
		r.Recorder.Eventf(eventType, corev1.EventTypeWarning, eventTypeUpdateStatusFailed, "Failed to update Broker's status: %v", err)
		return err
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, et *v1alpha1.EventType) error {
	et.Status.InitializeConditions()

	// 1. Verify the Broker exists.
	// 2. Verify the Broker is ready.

	if et.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	b, err := r.getBroker(ctx, et)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker", zap.Error(err))
		et.Status.MarkBrokerDoesNotExist()
		return err
	}
	et.Status.MarkBrokerExists()

	// Tell tracker to reconcile this EventType whenever the Broker changes.
	if err = r.tracker.Track(utils.ObjectRef(b, brokerGVK), et); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to Broker", zap.Error(err))
		return err
	}

	if !b.Status.IsReady() {
		logging.FromContext(ctx).Error("Broker is not ready", zap.String("broker", b.Name))
		et.Status.MarkBrokerNotReady()
		return nil
	}
	et.Status.MarkBrokerReady()

	return nil
}

// updateStatus updates the EventType's status.
func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.EventType) (*v1alpha1.EventType, error) {
	eventType, err := r.eventTypeLister.EventTypes(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(eventType.Status, desired.Status) {
		return eventType, nil
	}

	becomesReady := desired.Status.IsReady() && !eventType.Status.IsReady()

	// Don't modify the informers copy.
	existing := eventType.DeepCopy()
	existing.Status = desired.Status

	et, err := r.EventingClientSet.EventingV1alpha1().EventTypes(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(et.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Sugar().Infof("EventType %q became ready after %v", eventType.Name, duration)
		r.Recorder.Event(eventType, corev1.EventTypeNormal, eventTypeReadinessChanged, fmt.Sprintf("EventType %q became ready", eventType.Name))
		//r.StatsReporter.ReportServiceReady(eventType.Namespace, eventType.Name, duration) // TODO: stats
	}

	return et, err
}

// getBroker returns the Broker for EventType 'et' if it exists, otherwise it returns an error.
func (r *Reconciler) getBroker(ctx context.Context, et *v1alpha1.EventType) (*v1alpha1.Broker, error) {
	return r.brokerLister.Brokers(et.Namespace).Get(et.Spec.Broker)
}
