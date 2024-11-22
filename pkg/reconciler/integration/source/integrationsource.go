/*
Copyright 2024 The Knative Authors

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

package source

import (
	"context"
	"fmt"

	"knative.dev/eventing/pkg/reconciler/integration/source/resources"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/integrationsource"
	v1listers "knative.dev/eventing/pkg/client/listers/sources/v1"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1alpha1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	sourceReconciled       = "IntegrationSourceReconciled"
	containerSourceCreated = "ContainerSourceCreated"
	containerSourceUpdated = "ContainerSourceUpdated"
)

// Reconciler implements controller.Reconciler for ContainerSource resources.
type Reconciler struct {
	kubeClientSet     kubernetes.Interface
	eventingClientSet clientset.Interface

	containerSourceLister   v1listers.ContainerSourceLister
	integrationSourceLister listers.IntegrationSourceLister
}

// Check that our Reconciler implements Interface
var _ integrationsource.Interface = (*Reconciler)(nil)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason ContainerSourceReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, sourceReconciled, "IntegrationSource reconciled: \"%s/%s\"", namespace, name)
}

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha1.IntegrationSource) pkgreconciler.Event {

	_, err := r.reconcileContainerSource(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling ContainerSource", zap.Error(err))
		return err
	}

	return newReconciledNormal(source.Namespace, source.Name)
}

func (r *Reconciler) reconcileContainerSource(ctx context.Context, source *v1alpha1.IntegrationSource) (*v1.ContainerSource, error) {
	expected := resources.NewContainerSource(source)

	cs, err := r.containerSourceLister.ContainerSources(source.Namespace).Get(expected.Name)
	if apierrors.IsNotFound(err) {
		cs, err = r.eventingClientSet.SourcesV1().ContainerSources(source.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating new ContainerSource: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeNormal, containerSourceCreated, "ContainerSource created %q", cs.Name)
	} else if err != nil {
		return nil, fmt.Errorf("getting ContainerSource: %v", err)
	} else if !metav1.IsControlledBy(cs, source) {
		return nil, fmt.Errorf("ContainerSource %q is not owned by IntegrationSource %q", cs.Name, source.Name)
	} else if r.containerSourceSpecChanged(&cs.Spec, &expected.Spec) {
		cs.Spec = expected.Spec
		cs, err = r.eventingClientSet.SourcesV1().ContainerSources(source.Namespace).Update(ctx, cs, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating ContainerSource: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeNormal, containerSourceUpdated, "ContainerSource updated %q", cs.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing ContainerSource", zap.Any("ContainerSource", cs.ObjectMeta))
	}

	source.Status.PropagateContainerSourceStatus(&cs.Status)
	return cs, nil
}

func (r *Reconciler) containerSourceSpecChanged(have *v1.ContainerSourceSpec, want *v1.ContainerSourceSpec) bool {
	return !equality.Semantic.DeepDerivative(want, have)
}
