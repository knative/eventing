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

package flow

import (
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerAgentName = "flow-controller"

type reconciler struct {
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a flow controller.
func ProvideController(mrg manager.Manager) (controller.Controller, error) {
	// Setup a new controller to Reconcile Flows.
	c, err := controller.New(controllerAgentName, mrg, controller.Options{
		Reconciler: &reconciler{
			recorder: mrg.GetRecorder(controllerAgentName),
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Flow events and enqueue Flow object key.
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Flow{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// In addition to watching Flow objects, watch for objects that a Flow creates and own and when changes
	// are made to them, enqueue owning Flow object for reconcile loop.
	if err := c.Watch(&source.Kind{Type: &channelsv1alpha1.Channel{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Flow{}, IsController: true}); err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &channelsv1alpha1.Subscription{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Flow{}, IsController: true}); err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &feedsv1alpha1.Feed{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Flow{}, IsController: true}); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	return nil
}
