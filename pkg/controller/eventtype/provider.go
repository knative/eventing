/*
Copyright 2018 The Knative Authors

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
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
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
	controllerAgentName = "event-type-controller"
)

type reconciler struct {
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder
	logger     *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a flow controller.
func ProvideController(mgr manager.Manager) (controller.Controller, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	logger.With(zap.String("controller", controllerAgentName))

	// Setup a new controller to Reconcile Flows.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
			logger: logger,
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch EventType events and enqueue EventType object key.
	if err := c.Watch(&source.Kind{
		Type: &feedsv1alpha1.EventType{}},
		&handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// In addition to watching EventType objects, watch for Feeds, which use and 'pin' EventTypes.
	err = c.Watch(&source.Kind{Type: &feedsv1alpha1.Feed{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: feedToEventType{}})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

type feedToEventType struct{}

func (_ feedToEventType) Map(obj handler.MapObject) []reconcile.Request {
	feed, ok := obj.Object.(*feedsv1alpha1.Feed)
	if !ok {
		// This wasn't a Feed.
		return []reconcile.Request{}
	}
	etName := feed.Spec.Trigger.EventType

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: obj.Meta.GetNamespace(),
				Name:      etName,
			},
		},
	}
}
