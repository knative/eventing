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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgLogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/pingsource"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/pingsource/resources"
	"knative.dev/pkg/apis"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	pingReconciled              = "PingSourceReconciled"
	pingReadinessChanged        = "PingSourceReadinessChanged"
	pingUpdateStatusFailed      = "PingSourceUpdateStatusFailed"
	pingSourceDeploymentCreated = "PingSourceDeploymentCreated"
	pingSourceDeploymentUpdated = "PingSourceDeploymentUpdated"
	component                   = "pingsource"
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
	*reconciler.Base

	receiveAdapterImage string

	// listers index properties about resources
	pingLister       listers.PingSourceLister
	deploymentLister appsv1listers.DeploymentLister
	eventTypeLister  eventinglisters.EventTypeLister

	loggingContext context.Context
	sinkResolver   *resolver.URIResolver
	loggingConfig  *pkgLogging.Config
	metricsConfig  *metrics.ExporterOptions
}

// Check that our Reconciler implements controller.Reconciler
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

	ra, err := r.createReceiveAdapter(ctx, source, sinkURI)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return fmt.Errorf("creating receive adapter: %v", err)
	}
	source.Status.PropagateDeploymentAvailability(ra)

	_, err = r.reconcileEventType(ctx, source)
	if err != nil {
		source.Status.MarkNoEventType("EventTypeReconcileFailed", "")
		return fmt.Errorf("reconciling event types: %v", err)
	}
	source.Status.MarkEventType()

	return newReconciledNormal(source.Namespace, source.Name)
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.PingSource, sinkURI *apis.URL) (*appsv1.Deployment, error) {
	if err := checkResourcesStatus(src); err != nil {
		return nil, err
	}

	loggingConfig, err := pkgLogging.LoggingConfigToJson(r.loggingConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting logging config to JSON", zap.Any("receiveAdapter", err))
	}

	metricsConfig, err := metrics.MetricsOptionsToJson(r.metricsConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting metrics config to JSON", zap.Any("receiveAdapter", err))
	}

	adapterArgs := resources.Args{
		Image:         r.receiveAdapterImage,
		Source:        src,
		Labels:        resources.Labels(src.Name),
		SinkURI:       sinkURI,
		LoggingConfig: loggingConfig,
		MetricsConfig: metricsConfig,
	}
	expected := resources.MakeReceiveAdapter(&adapterArgs)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
		msg := "Deployment created"
		if err != nil {
			msg = fmt.Sprintf("Deployment created, error: %v", err)
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, pingSourceDeploymentCreated, "%s", msg)
		return ra, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by PingSource %q", ra.Name, src.Name)
	} else if podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
			return ra, err
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, pingSourceDeploymentUpdated, "Deployment %q updated", ra.Name)
		return ra, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
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

func (r *Reconciler) reconcileEventType(ctx context.Context, src *v1alpha1.PingSource) (*eventingv1alpha1.EventType, error) {
	sinkRef := src.Spec.Sink.GetRef()
	if sinkRef == nil {
		// Can't figure out the broker so return
		return nil, nil
	}
	expected := resources.MakeEventType(src)
	current, err := r.eventTypeLister.EventTypes(src.Namespace).Get(expected.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing event type", zap.Error(err))
		return nil, fmt.Errorf("getting event types: %v", err)
	}

	// Only create EventTypes for Broker sinks. But if there is an EventType and the src has a non-Broker sink
	// (possibly because it was updated), then we need to delete it.
	if sinkRef.Kind != "Broker" {
		if current != nil {
			if err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Delete(current.Name, &metav1.DeleteOptions{}); err != nil {
				logging.FromContext(ctx).Error("Error deleting existing event type", zap.Error(err), zap.Any("eventType", current))
				return nil, fmt.Errorf("deleting event type: %v", err)
			}
		}
		// No current and no error.
		return nil, nil
	}

	if current != nil {
		if equality.Semantic.DeepEqual(expected.Spec, current.Spec) {
			return current, nil
		}
		// EventTypes are immutable, delete it and create it again.
		if err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Delete(current.Name, &metav1.DeleteOptions{}); err != nil {
			logging.FromContext(ctx).Error("Error deleting existing event type", zap.Error(err), zap.Any("eventType", current))
			return nil, fmt.Errorf("deleting event type: %v", err)
		}
	}

	current, err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Create(expected)
	if err != nil {
		logging.FromContext(ctx).Error("Error creating event type", zap.Error(err), zap.Any("eventType", expected))
		return nil, fmt.Errorf("creating event type: %v", err)
	}
	logging.FromContext(ctx).Debug("EventType created", zap.Any("eventType", current))
	return current, nil
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
	logging.FromContext(r.loggingContext).Info("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
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
	logging.FromContext(r.loggingContext).Info("Update from metrics ConfigMap", zap.Any("ConfigMap", cfg))
}
