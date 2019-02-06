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
	"context"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultBroker = "default"
	knativeEventingAnnotation = "eventing.knative.dev/injection"

	// Name of the corev1.Events emitted from the reconciliation process
	brokerCreated = "BrokerCreated"
)

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

	// If the resource doesn't exist, we'll create it
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
			Namespace:    ns.Name,
			Name: defaultBroker,
			Labels:       brokerLabels(),
		},
	}
}

func brokerLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerForNamespace": "true",
	}
}
