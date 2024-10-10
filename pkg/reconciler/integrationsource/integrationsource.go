package integrationsource

import (
	"context"
	"fmt"
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
	"knative.dev/eventing/pkg/reconciler/integrationsource/resources"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	containerSourceCreated = "ContainerSourceCreated"
	containerSourceUpdated = "ContainerSourceUpdated"

	component = "integrationsource"
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

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha1.IntegrationSource) pkgreconciler.Event {

	_, err := r.reconcileContainerSource(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling ContainerSource", zap.Error(err))
		return err
	}

	return nil
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
		return nil, fmt.Errorf("ContainerSource %q is not owned by KameletSource %q", cs.Name, source.Name)
	} else if r.podSpecChanged(&cs.Spec.Template.Spec, &expected.Spec.Template.Spec) {
		cs.Spec.Template.Spec = expected.Spec.Template.Spec
		cs, err = r.eventingClientSet.SourcesV1().ContainerSources(source.Namespace).Update(ctx, cs, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating ContainerSource: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeNormal, containerSourceUpdated, "ContainerSource updated %q", cs.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing ContainerSource", zap.Any("ContainerSource", cs))
	}

	source.Status.PropagateContainerSourceStatus(&cs.Status)
	return cs, nil
}

func (r *Reconciler) podSpecChanged(have *corev1.PodSpec, want *corev1.PodSpec) bool {
	return !equality.Semantic.DeepDerivative(want, have)
}
