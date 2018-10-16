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

package sdk

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type KnativeReconciler interface {
	Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error)
	InjectClient(c client.Client) error
	InjectConfig(c *rest.Config) error
}

type Provider struct {
	AgentName string
	// Parent is a resource kind to reconcile with empty content. i.e. &v1.Parent{}
	Parent runtime.Object
	// Owns are dependent resources owned by the parent for which changes to
	// those resources cause the Parent to be re-reconciled. This is a list of
	// resources of kind with empty content. i.e. [&v1.Child{}]
	Owns []runtime.Object

	Reconciler KnativeReconciler
}

// ProvideController returns a controller for controller-runtime.
func (p *Provider) ProvideController(mgr manager.Manager) (controller.Controller, error) {
	// Setup a new controller to Reconcile Subscriptions.
	c, err := controller.New(p.AgentName, mgr, controller.Options{
		Reconciler: &Reconciler{
			provider: *p,
			recorder: mgr.GetRecorder(p.AgentName),
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Parent events and enqueue Parent object key.
	if err := c.Watch(&source.Kind{Type: p.Parent}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch and enqueue for owning obj key.
	for _, t := range p.Owns {
		if err := c.Watch(&source.Kind{Type: t},
			&handler.EnqueueRequestForOwner{OwnerType: p.Parent, IsController: true}); err != nil {
			return nil, err
		}
	}

	return c, nil
}
