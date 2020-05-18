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
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/trigger"
	listers "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	triggerNamespaceLabeled = "TriggerNamespaceLabeled"
)

type Reconciler struct {
	// eventingClientSet allows us to configure Eventing objects
	eventingClientSet clientset.Interface
	kubeClientSet     kubernetes.Interface

	brokerLister    listers.BrokerLister
	namespaceLister corev1listers.NamespaceLister
}

// Check that our Reconciler implements triggerreconciler.Interface
var _ triggerreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, t *v1beta1.Trigger) reconciler.Event {
	_, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil && apierrs.IsNotFound(err) {
		// https://github.com/knative/eventing/issues/2996
		// Ideally we'd default the Status of the Trigger during creation but we currently
		// can't, so watch for Triggers that do not have Broker for them and set their status.
		// This only addresses one part of the problem (missing the Broker outright, but
		// not the problem where the broker is misconfigured (wrong BrokerClass)), and hence
		// it still would not get reconciled.
		t.Status.MarkBrokerFailed("BrokerDoesNotExist", "Broker %q does not exist or there is no matching BrokerClass for it", t.Spec.Broker)

		_, needDefaultBroker := t.GetAnnotations()[v1beta1.InjectionAnnotation]
		if t.Spec.Broker == "default" && needDefaultBroker {
			if e := r.labelNamespace(ctx, t); e != nil {
				logging.FromContext(ctx).Errorw("Unable to label the namespace", zap.Error(e))
				return e
			}
		}
		return nil
	}
	return err
}

// labelNamespace will label namespace with knative-eventing-injection=enabled
func (r *Reconciler) labelNamespace(ctx context.Context, t *v1beta1.Trigger) reconciler.Event {
	current, err := r.namespaceLister.Get(t.Namespace)
	if err != nil {
		return err
	}
	current = current.DeepCopy()
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}
	current.Labels["knative-eventing-injection"] = "enabled"
	if _, err = r.kubeClientSet.CoreV1().Namespaces().Update(current); err != nil {
		return err
	}
	logging.FromContext(ctx).Infow("Labeled namespace", zap.String("namespace", t.Namespace))
	return reconciler.NewEvent(corev1.EventTypeNormal, triggerNamespaceLabeled,
		"Trigger namespaced labeled for injection: %q", t.Namespace)
}
