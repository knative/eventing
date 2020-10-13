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

package containersource

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/containersource"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1"
	"knative.dev/eventing/pkg/reconciler/containersource/resources"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	sourceReconciled   = "ContainerSourceReconciled"
	deploymentCreated  = "ContainerSourceDeploymentCreated"
	deploymentUpdated  = "ContainerSourceDeploymentUpdated"
	sinkBindingCreated = "ContainerSourceSinkBindingCreated"
	sinkBindingUpdated = "ContainerSourceSinkBindingUpdated"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason ContainerSourceReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, sourceReconciled, "ContainerSource reconciled: \"%s/%s\"", namespace, name)
}

// Reconciler implements controller.Reconciler for ContainerSource resources.
type Reconciler struct {
	kubeClientSet     kubernetes.Interface
	eventingClientSet clientset.Interface

	// listers index properties about resources
	containerSourceLister listers.ContainerSourceLister
	sinkBindingLister     listers.SinkBindingLister
	deploymentLister      appsv1listers.DeploymentLister
}

// Check that our Reconciler implements Interface
var _ containersource.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1.ContainerSource) pkgreconciler.Event {
	_, err := r.reconcileSinkBinding(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling SinkBinding", zap.Error(err))
		return err
	}

	_, err = r.reconcileReceiveAdapter(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling ReceiveAdapter", zap.Error(err))
		return err
	}

	return newReconciledNormal(source.Namespace, source.Name)
}

func (r *Reconciler) reconcileReceiveAdapter(ctx context.Context, source *v1.ContainerSource) (*appsv1.Deployment, error) {

	expected := resources.MakeDeployment(source)

	ra, err := r.deploymentLister.Deployments(expected.Namespace).Get(expected.Name)
	if apierrors.IsNotFound(err) {
		ra, err = r.kubeClientSet.AppsV1().Deployments(expected.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating new Deployment: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeNormal, deploymentCreated, "Deployment created %q", ra.Name)
	} else if err != nil {
		return nil, fmt.Errorf("getting Deployment: %v", err)
	} else if !metav1.IsControlledBy(ra, source) {
		return nil, fmt.Errorf("Deployment %q is not owned by ContainerSource %q", ra.Name, source.Name)
	} else if r.podSpecChanged(&ra.Spec.Template.Spec, &expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		ra, err = r.kubeClientSet.AppsV1().Deployments(expected.Namespace).Update(ctx, ra, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating Deployment: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeNormal, deploymentUpdated, "Deployment updated %q", ra.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing Deployment", zap.Any("Deployment", ra))
	}

	source.Status.PropagateReceiveAdapterStatus(ra)
	return ra, nil
}

func (r *Reconciler) reconcileSinkBinding(ctx context.Context, source *v1.ContainerSource) (*v1.SinkBinding, error) {

	expected := resources.MakeSinkBinding(source)

	sb, err := r.sinkBindingLister.SinkBindings(source.Namespace).Get(expected.Name)
	if apierrors.IsNotFound(err) {
		sb, err = r.eventingClientSet.SourcesV1().SinkBindings(source.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating new SinkBinding: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeNormal, sinkBindingCreated, "SinkBinding created %q", sb.Name)
	} else if err != nil {
		return nil, fmt.Errorf("getting SinkBinding: %v", err)
	} else if !metav1.IsControlledBy(sb, source) {
		return nil, fmt.Errorf("SinkBinding %q is not owned by ContainerSource %q", sb.Name, source.Name)
	} else if r.sinkBindingSpecChanged(&sb.Spec, &expected.Spec) {
		sb.Spec = expected.Spec
		sb, err = r.eventingClientSet.SourcesV1().SinkBindings(source.Namespace).Update(ctx, sb, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating SinkBinding: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeNormal, sinkBindingUpdated, "SinkBinding updated %q", sb.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing SinkBinding", zap.Any("SinkBinding", sb))
	}

	source.Status.PropagateSinkBindingStatus(&sb.Status)
	return sb, nil
}

func (r *Reconciler) podSpecChanged(have *corev1.PodSpec, want *corev1.PodSpec) bool {
	// TODO this won't work, SinkBinding messes with this. n3wscott working on a fix.
	return !equality.Semantic.DeepDerivative(want, have)
}

func (r *Reconciler) sinkBindingSpecChanged(have *v1.SinkBindingSpec, want *v1.SinkBindingSpec) bool {
	return !equality.Semantic.DeepDerivative(want, have)
}
