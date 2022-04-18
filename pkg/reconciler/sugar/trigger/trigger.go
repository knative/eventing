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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	sugarconfig "knative.dev/eventing/pkg/apis/sugar"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	listers "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/eventing/pkg/reconciler/sugar/resources"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated = "BrokerCreated"
)

type Reconciler struct {
	// eventingClientSet allows us to configure Eventing objects
	eventingClientSet clientset.Interface
	brokerLister      listers.BrokerLister
}

// Check that our Reconciler implements triggerreconciler.Interface
var _ triggerreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, t *v1.Trigger) reconciler.Event {

	cfg := sugarconfig.FromContext(ctx)

	selector, err := metav1.LabelSelectorAsSelector(cfg.TriggerSelector)
	if err != nil {
		return fmt.Errorf("invalid label selector for triggers: %w", err)
	}
	if !selector.Matches(kubelabels.Set(t.GetLabels())) {
		logging.FromContext(ctx).Debugf("Sugar Controller disabled for Trigger:%s in configmap 'config-sugar'", t.Name)
		return nil
	} else {
		logging.FromContext(ctx).Debugf("Sugar Controller enabled for Trigger:%s in configmap 'config-sugar'", t.Name)
	}

	_, err = r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		_, err = r.eventingClientSet.EventingV1().Brokers(t.Namespace).Create(
			ctx, resources.MakeBroker(t.Namespace, t.Spec.Broker), metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("Unable to create Broker: %w", err)
		}
		return reconciler.NewEvent(corev1.EventTypeNormal, brokerCreated,
			fmt.Sprintf("Default eventing.knative.dev Broker %q created.", t.Spec.Broker))
	} else if err != nil {
		return fmt.Errorf("Unable to list Brokers: %w", err)
	}

	return nil
}
