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

package trigger

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	triggerReconcileFailed  = "TriggerReconcileFailed"
	triggerNamespaceLabeled = "TriggerNamespaceLabeled"
)

type Reconciler struct {
	*reconciler.Base

	triggerLister   listers.TriggerLister
	brokerLister    listers.BrokerLister
	namespaceLister corev1listers.NamespaceLister
}

var brokerGVK = v1alpha1.SchemeGroupVersion.WithKind("Broker")

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Trigger resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the Trigger resource with this namespace/name.
	original, err := r.triggerLister.Triggers(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("trigger key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	trigger := original.DeepCopy()

	// Reconcile this copy of the Trigger and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, trigger)
	if reconcileErr != nil {
		r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerReconcileFailed, "Trigger reconciliation failed: %v", reconcileErr)
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, t *v1alpha1.Trigger) error {
	// Check a triggers annotations and if missing a broker, inject the namespace.

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	_, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker", zap.Error(err))
		if apierrs.IsNotFound(err) {
			_, needDefaultBroker := t.GetAnnotations()[v1alpha1.InjectionAnnotation]
			if t.Spec.Broker == "default" && needDefaultBroker {
				if e := r.labelNamespace(ctx, t); e != nil {
					logging.FromContext(ctx).Error("Unable to label the namespace", zap.Error(e))
					return e
				}
			}
			return nil
		}
		return err
	}
	return nil
}

// labelNamespace will label namespace with knative-eventing-injection=enabled
func (r *Reconciler) labelNamespace(ctx context.Context, t *v1alpha1.Trigger) error {
	current, err := r.namespaceLister.Get(t.Namespace)
	if err != nil {
		return err
	}
	current = current.DeepCopy()
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}
	current.Labels["knative-eventing-injection"] = "enabled"
	if _, err = r.KubeClientSet.CoreV1().Namespaces().Update(current); err != nil {
		logging.FromContext(ctx).Error("Unable to update the namespace", zap.Error(err))
		return err
	}
	logging.FromContext(ctx).Info("Labeled namespace", zap.String("namespace", t.Namespace))
	r.Recorder.Eventf(t, corev1.EventTypeNormal, triggerNamespaceLabeled, "Trigger namespaced labeled for injection: %q", t.Namespace)
	return nil
}
