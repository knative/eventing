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

package apiserver

import (
	"context"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	eventsclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing/pkg/reconciler"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "ApiServerSource"

	controllerAgentName = "apiserver-source-adapter-controller"
	updateEventType     = "dev.knative.apiserver.object.update"
	deleteEventType     = "dev.knative.apiserver.object.delete"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	informer cache.SharedInformer,
	lister cache.GenericLister,
	eventsclient eventsclient.Client,
	controlled bool) *controller.Impl {

	r := &Reconciler{
		Base:         reconciler.NewBase(opt, controllerAgentName),
		lister:       lister,
		eventsClient: eventsclient,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.Logger.Info("Setting up event handlers")

	if controlled {
		informer.AddEventHandler(reconciler.Handler(impl.EnqueueControllerOf))
	} else {
		informer.AddEventHandler(reconciler.Handler(impl.Enqueue))
	}
	return impl
}

// Reconciler reconciles an ApiServerSource object
type Reconciler struct {
	*reconciler.Base

	eventsClient eventsclient.Client
	lister       cache.GenericLister
}

// Reconcile sends a cloud event corresponding to the given key
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the resource with this namespace/name
	original, err := r.lister.ByNamespace(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		r.Logger.Error("resource key in work queue no longer exists", zap.Any("key", key))
		return nil
	} else if err != nil {
		return err
	}

	object := original.(*duckv1alpha1.AddressableType)

	eventType := updateEventType
	timestamp := object.GetCreationTimestamp()
	if object.GetDeletionTimestamp() != nil {
		eventType = deleteEventType
		timestamp = *object.GetDeletionTimestamp()
	}

	objectRef := corev1.ObjectReference{
		APIVersion: object.APIVersion,
		Kind:       object.Kind,
		Name:       object.GetName(),
		Namespace:  object.GetNamespace(),
	}

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:     string(object.GetUID()),
			Type:   eventType,
			Source: *types.ParseURLRef(object.GetSelfLink()),
			Time:   &types.Timestamp{Time: timestamp.Time},
		}.AsV02(),
		Data: objectRef,
	}

	if _, err := r.eventsClient.Send(ctx, event); err != nil {
		r.Logger.Error("failed to send cloudevent (retrying)", err)

		return err
	}

	return nil
}
