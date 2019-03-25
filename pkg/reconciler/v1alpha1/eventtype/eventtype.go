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

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "eventtype-controller"

	// Name of the corev1.Events emitted from the reconciliation process.
	eventTypeReconciled         = "EventTypeReconciled"
	eventTypeReconcileFailed    = "EventTypeReconcileFailed"
	eventTypeUpdateStatusFailed = "EventTypeUpdateStatusFailed"
)

type reconciler struct {
	client        client.Client
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder

	logger *zap.Logger
}

// Verify the struct implements reconcile.Reconciler.
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a function that returns an EventType controller.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	// Setup a new controller to Reconcile EventTypes.
	r := &reconciler{
		recorder: mgr.GetRecorder(controllerAgentName),
		logger:   logger,
	}
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return nil, err
	}

	// Watch EventTypes.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.EventType{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch for Broker changes. E.g. if the Broker is deleted, we need to reconcile its EventTypes again.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Broker{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &mapBrokerToEventTypes{r: r}}); err != nil {
		return nil, err
	}

	return c, nil
}

// mapBrokerToEventTypes maps Broker changes to all the EventTypes that correspond to that Broker.
type mapBrokerToEventTypes struct {
	r *reconciler
}

func (b *mapBrokerToEventTypes) Map(o handler.MapObject) []reconcile.Request {
	ctx := context.Background()
	eventTypes := make([]reconcile.Request, 0)

	opts := &client.ListOptions{
		Namespace: o.Meta.GetNamespace(),
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}
	for {
		etl := &v1alpha1.EventTypeList{}
		if err := b.r.client.List(ctx, opts, etl); err != nil {
			b.r.logger.Error("Error listing EventTypes when Broker changed. Some EventTypes may not be reconciled.", zap.Error(err), zap.Any("broker", o))
			return eventTypes
		}

		for _, et := range etl.Items {
			if et.Spec.Broker == o.Meta.GetName() {
				eventTypes = append(eventTypes, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: et.Namespace,
						Name:      et.Name,
					},
				})
			}
		}
		if etl.Continue != "" {
			opts.Raw.Continue = etl.Continue
		} else {
			return eventTypes
		}
	}
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the EventType resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

	eventType := &v1alpha1.EventType{}
	err := r.client.Get(ctx, request.NamespacedName, eventType)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find EventType")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Could not get EventType", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Reconcile this copy of the EventType and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, eventType)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling EventType", zap.Error(reconcileErr))
		r.recorder.Eventf(eventType, corev1.EventTypeWarning, eventTypeReconcileFailed, "EventType reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("EventType reconciled")
		r.recorder.Event(eventType, corev1.EventTypeNormal, eventTypeReconciled, "EventType reconciled")
	}

	if _, err = r.updateStatus(eventType); err != nil {
		logging.FromContext(ctx).Error("Failed to update EventType status", zap.Error(err))
		r.recorder.Eventf(eventType, corev1.EventTypeWarning, eventTypeUpdateStatusFailed, "Failed to update EventType's status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready
	return reconcile.Result{}, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, et *v1alpha1.EventType) error {
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

	if !b.Status.IsReady() {
		logging.FromContext(ctx).Error("Broker is not ready", zap.String("broker", b.Name))
		et.Status.MarkBrokerNotReady()
		return fmt.Errorf("broker %q not ready", b.Name)
	}
	et.Status.MarkBrokerReady()

	return nil
}

// updateStatus updates the event type's status.
func (r *reconciler) updateStatus(eventType *v1alpha1.EventType) (*v1alpha1.EventType, error) {
	ctx := context.TODO()
	objectKey := client.ObjectKey{Namespace: eventType.Namespace, Name: eventType.Name}
	latestEventType := &v1alpha1.EventType{}

	if err := r.client.Get(ctx, objectKey, latestEventType); err != nil {
		return nil, err
	}

	if equality.Semantic.DeepEqual(latestEventType.Status, eventType.Status) {
		return eventType, nil
	}

	latestEventType.Status = eventType.Status
	if err := r.client.Status().Update(ctx, latestEventType); err != nil {
		return nil, err
	}

	return latestEventType, nil
}

// getBroker returns the Broker for EventType 'et' if exists, otherwise it returns an error.
func (r *reconciler) getBroker(ctx context.Context, et *v1alpha1.EventType) (*v1alpha1.Broker, error) {
	b := &v1alpha1.Broker{}
	name := types.NamespacedName{
		Namespace: et.Namespace,
		Name:      et.Spec.Broker,
	}
	err := r.client.Get(ctx, name, b)
	return b, err
}
