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

package namespace

import (
	"context"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/logging"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	defaultBroker             = "default"
	knativeEventingAnnotation = "eventing.knative.dev/inject"

	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated = "BrokerCreated"
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

		// Watch Namespaces.
		if err = c.Watch(&source.Kind{Type: &v1.Namespace{}}, &handler.EnqueueRequestForObject{}); err != nil {
			return nil, err
		}

		// Watch all the resources that this reconciler reconciles.
		for _, t := range []runtime.Object{ &v1alpha1.Broker{} } {
			err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &namespaceMapper{}});
			if err != nil {
				return nil, err
			}
		}

		return c, nil
	}
}

type namespaceMapper struct{}

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

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Trigger resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

	ns := &corev1.Namespace{}
	err := r.client.Get(context.TODO(), request.NamespacedName, ns)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Namespace")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Could not Get Namespace", zap.Error(err))
		return reconcile.Result{}, err
	}

	if ns.Annotations[knativeEventingAnnotation] != "true" {
		logging.FromContext(ctx).Debug("Not reconciling Namespace")
		return reconcile.Result{}, nil
	}

	// Reconcile this copy of the Trigger and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, ns)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Namespace", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("Namespace reconciled")
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, ns *corev1.Namespace) error {
	if ns.DeletionTimestamp != nil {
		return nil
	}

	_, err := r.reconcileBroker(ctx, ns)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile broker for the namespace", zap.Error(err))
		return err
	}

	return nil
}

func (r *reconciler) getBroker(ctx context.Context, ns *corev1.Namespace) (*v1alpha1.Broker, error) {
	b := &v1alpha1.Broker{}
	name := types.NamespacedName{
		Namespace: ns.Name,
		Name:      defaultBroker,
	}
	err := r.client.Get(ctx, name, b)
	return b, err
}

func (r *reconciler) reconcileBroker(ctx context.Context, ns *corev1.Namespace) (*v1alpha1.Broker, error) {
	current, err := r.getBroker(ctx, ns)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		b := newBroker(ns)
		err = r.client.Create(ctx, b)
		if err != nil {
			return nil, err
		}
		r.recorder.Event(ns, corev1.EventTypeNormal, brokerCreated, "Default eventing.knative.dev Broker created.")
		return b, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

func newBroker(ns *corev1.Namespace) *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.Name,
			Name:      defaultBroker,
			Labels:    brokerLabels(),
		},
	}
}

func brokerLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerForNamespace": "true",
	}
}
