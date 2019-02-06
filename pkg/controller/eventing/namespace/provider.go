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

package namespace

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/api/core/v1"
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
	controllerAgentName = "knative-eventing-namespace-controller"
)

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder

	logger *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a function that returns a Broker controller.
func ProvideController(logger *zap.Logger) func(manager.Manager) (controller.Controller, error) {
	return func(mgr manager.Manager) (controller.Controller, error) {
		// Setup a new controller to Reconcile Brokers.
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

		// Watch Subscription events and enqueue Subscription object key.
		if err = c.Watch(&source.Kind{Type: &v1.Namespace{}}, &handler.EnqueueRequestForObject{}); err != nil {
			return nil, err
		}

		if err = c.Watch(&source.Kind{Type: &v1alpha1.Broker{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &namespaceMapper{}}); err != nil {
			return nil, err
		}

		return c, nil
	}
}

type namespaceMapper struct {}
var _ handler.Mapper = &namespaceMapper{}

func (namespaceMapper) Map(o handler.MapObject) []reconcile.Request {
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: "",
				Name:      o.Meta.GetNamespace(),
			},
		},
	}
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}
