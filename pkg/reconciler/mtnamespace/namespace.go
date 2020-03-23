/*
Copyright 2020 The Knative Authors

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

package mtnamespace

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/mtnamespace/resources"
)

const (
	namespaceReconciled       = "NamespaceReconciled"
	namespaceReconcileFailure = "NamespaceReconcileFailure"

	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated = "BrokerCreated"
)

type Reconciler struct {
	eventingClientSet clientset.Interface

	// listers index properties about resources
	namespaceLister corev1listers.NamespaceLister
	brokerLister    eventinglisters.BrokerLister

	recorder record.EventRecorder
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Namespace resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the namespace resource with this namespace/name
	original, err := r.namespaceLister.Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("namespace key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	if original.Labels[resources.InjectionLabelKey] == resources.InjectionDisabledLabelValue {
		logging.FromContext(ctx).Debug("Not reconciling Namespace")
		return nil
	}

	// Don't modify the informers copy
	ns := original.DeepCopy()

	// Reconcile this copy of the Namespace.
	reconcileErr := r.reconcile(ctx, ns)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Namespace", zap.Error(reconcileErr))
		r.recorder.Eventf(ns, corev1.EventTypeWarning, namespaceReconcileFailure, "Failed to reconcile Namespace: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("Namespace reconciled")
		r.recorder.Eventf(ns, corev1.EventTypeNormal, namespaceReconciled, "Namespace reconciled: %q", ns.Name)
	}

	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, ns *corev1.Namespace) error {
	if ns.DeletionTimestamp != nil {
		return nil
	}

	if _, err := r.reconcileBroker(ctx, ns); err != nil {
		return fmt.Errorf("broker: %v", err)
	}

	return nil
}

// reconcileBroker reconciles the default Broker for the Namespace 'ns'.
func (r *Reconciler) reconcileBroker(ctx context.Context, ns *corev1.Namespace) (*v1beta1.Broker, error) {
	current, err := r.brokerLister.Brokers(ns.Name).Get(resources.DefaultBrokerName)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		b := resources.MakeBroker(ns)
		b, err = r.eventingClientSet.EventingV1beta1().Brokers(ns.Name).Create(b)
		if err != nil {
			return nil, err
		}
		r.recorder.Event(ns, corev1.EventTypeNormal, brokerCreated,
			"Default eventing.knative.dev Broker created.")
		return b, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}
