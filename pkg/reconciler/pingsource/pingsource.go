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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	pkgLogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/pingsource"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	pingSourceDeploymentCreated = "PingSourceDeploymentCreated"
	pingSourceDeploymentUpdated = "PingSourceDeploymentUpdated"
	pingSourceDeploymentDeleted = "PingSourceDeploymentDeleted"
	component                   = "pingsource"
	mtadapterName               = "pingsource-mt-adapter"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason PingSourceReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "PingSourceReconciled", "PingSource reconciled: \"%s/%s\"", namespace, name)
}

func newWarningSinkNotFound(sink *duckv1.Destination) pkgreconciler.Event {
	b, _ := json.Marshal(sink)
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "SinkNotFound", "Sink not found: %s", string(b))
}

type Reconciler struct {
	kubeClientSet kubernetes.Interface

	receiveMTAdapterImage string

	// listers index properties about resources
	pingLister       listers.PingSourceLister
	deploymentLister appsv1listers.DeploymentLister

	// tracking mt adapter deployment changes
	tracker tracker.Interface

	loggingContext context.Context
	sinkResolver   *resolver.URIResolver
	loggingConfig  *pkgLogging.Config
	metricsConfig  *metrics.ExporterOptions
}

// Check that our Reconciler implements ReconcileKind
var _ pingsourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha1.PingSource) pkgreconciler.Event {
	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this PingSource is deleted.
	// 3. Create the EventType that it can emit.
	//     - Will be garbage collected by K8s when this PingSource is deleted.
	source.Status.ObservedGeneration = source.Generation

	source.Status.InitializeConditions()

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
	d, err := r.reconcileMTReceiveAdapter(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the mt receive adapter", zap.Error(err))
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
		logging.FromContext(ctx).Error("Unable to track the deployment", zap.Error(err))
		return err
	}

	source.Status.CloudEventAttributes = []duckv1.CloudEventAttributes{{
		Type:   v1alpha1.PingSourceEventType,
		Source: v1alpha1.PingSourceSource(source.Namespace, source.Name),
	}}

	return newReconciledNormal(source.Namespace, source.Name)
}

func (r *Reconciler) reconcileMTReceiveAdapter(ctx context.Context, source *v1alpha1.PingSource) (*appsv1.Deployment, error) {
	if err := checkResourcesStatus(source); err != nil {
		return nil, err
	}

	args := resources.MTArgs{
		ServiceAccountName: mtadapterName,
		MTAdapterName:      mtadapterName,
		Image:              r.receiveMTAdapterImage,
	}
	expected := resources.MakeMTReceiveAdapter(args)

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
		logging.FromContext(ctx).Debug("Reusing existing cluster-scoped deployment", zap.Any("deployment", d))
	}
	return d, nil
}

func checkResourcesStatus(src *v1alpha1.PingSource) error {
	for _, rsrc := range []struct {
		key   string
		field string
	}{{
		key:   "Request.CPU",
		field: src.Spec.Resources.Requests.ResourceCPU,
	}, {
		key:   "Request.Memory",
		field: src.Spec.Resources.Requests.ResourceMemory,
	}, {
		key:   "Limit.CPU",
		field: src.Spec.Resources.Limits.ResourceCPU,
	}, {
		key:   "Limit.Memory",
		field: src.Spec.Resources.Limits.ResourceMemory,
	}} {
		// In the event the field isn't specified, we assign a default in the receive_adapter
		if rsrc.field != "" {
			if _, err := resource.ParseQuantity(rsrc.field); err != nil {
				src.Status.MarkResourcesIncorrect("Incorrect Resource", "%s: %q, Error: %s", rsrc.key, rsrc.field, err)
				return fmt.Errorf("incorrect resource specification, %s: %q: %v", rsrc.key, rsrc.field, err)
			}
		}
	}
	src.Status.MarkResourcesCorrect()
	return nil
}

func podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
	if !equality.Semantic.DeepDerivative(newPodSpec, oldPodSpec) {
		return true
	}
	if len(oldPodSpec.Containers) != len(newPodSpec.Containers) {
		return true
	}
	for i := range newPodSpec.Containers {
		if !equality.Semantic.DeepEqual(newPodSpec.Containers[i].Env, oldPodSpec.Containers[i].Env) {
			return true
		}
	}
	return false
}

// TODO determine how to push the updated logging config to existing data plane Pods.
func (r *Reconciler) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	logcfg, err := pkgLogging.NewConfigFromConfigMap(cfg)
	if err != nil {
		logging.FromContext(r.loggingContext).Warn("failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.loggingConfig = logcfg
	logging.FromContext(r.loggingContext).Debug("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
}

// TODO determine how to push the updated metrics config to existing data plane Pods.
func (r *Reconciler) UpdateFromMetricsConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	r.metricsConfig = &metrics.ExporterOptions{
		Domain:    metrics.Domain(),
		Component: component,
		ConfigMap: cfg.Data,
	}
	logging.FromContext(r.loggingContext).Debug("Update from metrics ConfigMap", zap.Any("ConfigMap", cfg))
}
