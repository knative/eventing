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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/mtnamespace/resources"
	namespacereconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/namespace"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated = "BrokerCreated"
)

type labelFilter func(labels map[string]string) bool

type Reconciler struct {
	eventingClientSet clientset.Interface

	filter labelFilter

	// listers index properties about resources
	brokerLister eventinglisters.BrokerLister
}

// Check that our Reconciler implements namespacereconciler.Interface
var _ namespacereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, ns *corev1.Namespace) pkgreconciler.Event {
	if r.filter(ns.Labels) {
		logging.FromContext(ctx).Debug("Not reconciling Namespace")
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
		// we want the event created in the namespace, and while ns is a cluster
		// wide object, if don't do this we'll end with the event created
		// in the default namespace, which is a bad UX in our case.
		ns.SetNamespace(ns.Name)
		controller.GetEventRecorder(ctx).Event(ns, corev1.EventTypeNormal, brokerCreated,
			"Default eventing.knative.dev Broker created.")
		return b, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}
