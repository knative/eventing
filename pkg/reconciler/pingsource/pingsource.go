/*
Copyright 2021 The Knative Authors

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

package pingsource

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/adapter/mtping"
	"knative.dev/eventing/pkg/adapter/v2"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/pingsource"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	pingSourceDeploymentUpdated = "PingSourceDeploymentUpdated"

	component     = "pingsource"
	mtcomponent   = "pingsource-mt-adapter"
	mtadapterName = "pingsource-mt-adapter"
	containerName = "dispatcher"
)

func newWarningSinkNotFound(sink *duckv1.Destination) pkgreconciler.Event {
	b, _ := json.Marshal(sink)
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "SinkNotFound", "Sink not found: %s", string(b))
}

type Reconciler struct {
	kubeClientSet kubernetes.Interface

	// tracking mt adapter deployment changes
	tracker tracker.Interface

	sinkResolver *resolver.URIResolver

	// config accessor for observability/logging/tracing
	configAcc reconcilersource.ConfigAccessor

	// Leader election configuration for the mt receive adapter
	leConfig string
}

// Check that our Reconciler implements ReconcileKind
var _ pingsourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *sourcesv1.PingSource) pkgreconciler.Event {
	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this PingSource is deleted.
	// 3. Create the EventType that it can emit.
	//     - Will be garbage collected by K8s when this PingSource is deleted.

	dest := source.Spec.Sink.DeepCopy()
	if dest.Ref != nil {
		// To call URIFromDestination(), dest.Ref must have a Namespace. If there is
		// no Namespace defined in dest.Ref, we will use the Namespace of the source
		// as the Namespace of dest.Ref.
		if dest.Ref.Namespace == "" {
			//TODO how does this work with deprecated fields
			dest.Ref.Namespace = source.GetNamespace()
		}
	}

	sinkURI, err := r.sinkResolver.URIFromDestinationV1(ctx, *dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return newWarningSinkNotFound(dest)
	}
	source.Status.MarkSink(sinkURI)

	// Make sure the global mt receive adapter is running
	d, err := r.reconcileReceiveAdapter(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Errorw("Unable to reconcile the receive adapter", zap.Error(err))
		return err
	}
	source.Status.PropagateDeploymentAvailability(d)

	// Tell tracker to reconcile this PingSource whenever the deployment changes
	err = r.tracker.TrackReference(tracker.Reference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  d.Namespace,
		Name:       d.Name,
	}, source)

	if err != nil {
		logging.FromContext(ctx).Errorw("Unable to track the deployment", zap.Error(err))
		return err
	}

	source.Status.CloudEventAttributes = []duckv1.CloudEventAttributes{{
		Type:   sourcesv1.PingSourceEventType,
		Source: sourcesv1.PingSourceSource(source.Namespace, source.Name),
	}}

	return nil
}

func (r *Reconciler) reconcileReceiveAdapter(ctx context.Context, source *sourcesv1.PingSource) (*appsv1.Deployment, error) {
	args := resources.Args{
		ConfigEnvVars:   r.configAcc.ToEnvVars(),
		LeConfig:        r.leConfig,
		NoShutdownAfter: mtping.GetNoShutDownAfterValue(),
		SinkTimeout:     adapter.GetSinkTimeout(logging.FromContext(ctx)),
	}
	expected := resources.MakeReceiveAdapterEnvVar(args)

	d, err := r.kubeClientSet.AppsV1().Deployments(system.Namespace()).Get(ctx, mtadapterName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logging.FromContext(ctx).Errorw("PingSource adapter deployment doesn't exist", zap.Error(err))
			return nil, err
		}
		return nil, fmt.Errorf("error getting mt adapter deployment %v", err)
	} else if update, c := needsUpdating(ctx, &d.Spec, expected); update {
		c.Env = expected

		if zero(d.Spec.Replicas) {
			d.Spec.Replicas = pointer.Int32Ptr(1)
		}

		if d, err = r.kubeClientSet.AppsV1().Deployments(system.Namespace()).Update(ctx, d, metav1.UpdateOptions{}); err != nil {
			return d, err
		}
		controller.GetEventRecorder(ctx).Event(source, corev1.EventTypeNormal, pingSourceDeploymentUpdated, "PingSource adapter deployment updated")
		return d, nil
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing cluster-scoped deployment", zap.Any("deployment", d))
	}
	return d, nil
}

func needsUpdating(ctx context.Context, oldDeploymentSpec *appsv1.DeploymentSpec, newEnvVars []corev1.EnvVar) (bool, *corev1.Container) {
	// We just care about the environment of the dispatcher container
	oldPodSpec := &oldDeploymentSpec.Template.Spec
	container := findContainer(oldPodSpec, containerName)
	if container == nil {
		logging.FromContext(ctx).Errorf("invalid %s deployment: missing the %s container", mtadapterName, containerName)
		return false, nil
	}

	return zero(oldDeploymentSpec.Replicas) || !equality.Semantic.DeepEqual(container.Env, newEnvVars), container
}

func findContainer(podSpec *corev1.PodSpec, name string) *corev1.Container {
	for i, container := range podSpec.Containers {
		if container.Name == name {
			return &podSpec.Containers[i]
		}
	}
	return nil
}

func zero(i *int32) bool {
	return i != nil && *i == 0
}
