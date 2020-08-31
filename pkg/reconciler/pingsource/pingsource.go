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
	"k8s.io/client-go/kubernetes"

	appsv1listers "k8s.io/client-go/listers/apps/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/adapter/mtping"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1alpha2"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	pingSourceDeploymentCreated = "PingSourceDeploymentCreated"
	pingSourceDeploymentUpdated = "PingSourceDeploymentUpdated"

	component     = "pingsource"
	mtcomponent   = "pingsource-mt-adapter"
	mtadapterName = "pingsource-mt-adapter"
)

func newWarningSinkNotFound(sink *duckv1.Destination) pkgreconciler.Event {
	b, _ := json.Marshal(sink)
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "SinkNotFound", "Sink not found: %s", string(b))
}

type Reconciler struct {
	kubeClientSet kubernetes.Interface

	receiveAdapterImage string

	// listers index properties about resources
	pingLister       listers.PingSourceLister
	deploymentLister appsv1listers.DeploymentLister

	// tracking mt adapter deployment changes
	tracker tracker.Interface

	loggingContext context.Context
	sinkResolver   *resolver.URIResolver

	// Leader election configuration for the mt receive adapter
	leConfig string
	configs  *reconcilersource.ConfigWatcher
}

// Check that our Reconciler implements ReconcileKind
var _ pingsourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha2.PingSource) pkgreconciler.Event {
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

	sinkURI, err := r.sinkResolver.URIFromDestinationV1(*dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return newWarningSinkNotFound(dest)
	}
	source.Status.MarkSink(sinkURI)

	// The webhook does not allow for invalid schedules to be posted.
	// TODO: remove MarkSchedule
	source.Status.MarkSchedule()

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
		Type:   v1alpha2.PingSourceEventType,
		Source: v1alpha2.PingSourceSource(source.Namespace, source.Name),
	}}

	return nil
}

func (r *Reconciler) reconcileReceiveAdapter(ctx context.Context, source *v1alpha2.PingSource) (*appsv1.Deployment, error) {
	loggingConfig, err := logging.LoggingConfigToJson(r.configs.LoggingConfig())
	if err != nil {
		logging.FromContext(ctx).Errorw("error while converting logging config to JSON", zap.Any("receiveAdapter", err))
	}

	metricsConfig, err := metrics.MetricsOptionsToJson(r.configs.MetricsConfig())
	if err != nil {
		logging.FromContext(ctx).Errorw("error while converting metrics config to JSON", zap.Any("receiveAdapter", err))
	}

	args := resources.Args{
		ServiceAccountName: mtadapterName,
		AdapterName:        mtadapterName,
		Image:              r.receiveAdapterImage,
		LoggingConfig:      loggingConfig,
		MetricsConfig:      metricsConfig,
		LeConfig:           r.leConfig,
		NoShutdownAfter:    mtping.GetNoShutDownAfterValue(),
	}
	expected := resources.MakeReceiveAdapter(args)

	d, err := r.deploymentLister.Deployments(system.Namespace()).Get(mtadapterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			d, err := r.kubeClientSet.AppsV1().Deployments(system.Namespace()).Create(expected)
			if err != nil {
				controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeWarning, pingSourceDeploymentCreated, "Cluster-scoped deployment not created (%v)", err)
				return nil, err
			}
			controller.GetEventRecorder(ctx).Event(source, corev1.EventTypeNormal, pingSourceDeploymentCreated, "Cluster-scoped deployment created")
			return d, nil
		}
		return nil, fmt.Errorf("error getting mt adapter deployment %v", err)
	} else if podSpecChanged(d.Spec.Template.Spec, expected.Spec.Template.Spec) {
		d.Spec.Template.Spec = expected.Spec.Template.Spec
		if d, err = r.kubeClientSet.AppsV1().Deployments(system.Namespace()).Update(d); err != nil {
			return d, err
		}
		controller.GetEventRecorder(ctx).Event(source, corev1.EventTypeNormal, pingSourceDeploymentUpdated, "Cluster-scoped deployment updated")
		return d, nil
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing cluster-scoped deployment", zap.Any("deployment", d))
	}
	return d, nil
}

func podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
	// We really care about the fields we set and ignore the test.
	return !equality.Semantic.DeepDerivative(newPodSpec, oldPodSpec)
}
