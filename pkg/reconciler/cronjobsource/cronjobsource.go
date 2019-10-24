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

package cronjobsource

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/robfig/cron"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/cronjobsource/resources"
	"knative.dev/pkg/controller"
	pkgLogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	cronJobReconciled              = "CronJobSourceReconciled"
	cronJobReadinessChanged        = "CronJobSourceReadinessChanged"
	cronJobUpdateStatusFailed      = "CronJobSourceUpdateStatusFailed"
	cronJobSourceDeploymentCreated = "CronJobSurceDeploymentCreated"
	cronJobSourceDeploymentUpdated = "CronJobSourceDeploymentUpdated"
	component                      = "cronjobsource"
)

type Reconciler struct {
	*reconciler.Base

	receiveAdapterImage string

	// listers index properties about resources
	cronjobLister    listers.CronJobSourceLister
	deploymentLister appsv1listers.DeploymentLister
	eventTypeLister  eventinglisters.EventTypeLister

	loggingContext context.Context
	sinkReconciler *duck.SinkReconciler
	loggingConfig  *pkgLogging.Config
	metricsConfig  *metrics.ExporterOptions
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CronJobSource
// resource with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the CronJobSource resource with this namespace/name
	original, err := r.cronjobLister.CronJobSources(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("CronJobSource key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	cronjob := original.DeepCopy()

	// Reconcile this copy of the CronJobSource and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, cronjob)
	if err != nil {
		logging.FromContext(ctx).Warn("Error reconciling CronJobSource", zap.Error(err))
	} else {
		logging.FromContext(ctx).Debug("CronJobSource reconciled")
		r.Recorder.Eventf(cronjob, corev1.EventTypeNormal, cronJobReconciled, `CronJobSource reconciled: "%s/%s"`, cronjob.Namespace, cronjob.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, cronjob.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the CronJobSource", zap.Error(err))
		r.Recorder.Eventf(cronjob, corev1.EventTypeWarning, cronJobUpdateStatusFailed, "Failed to update CronJobSource's status: %v", err)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return err
}

func (r *Reconciler) reconcile(ctx context.Context, cronjob *v1alpha1.CronJobSource) error {
	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
	// 3. Create the EventType that it can emit.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
	cronjob.Status.ObservedGeneration = cronjob.Generation

	cronjob.Status.InitializeConditions()

	sinkObjRef := cronjob.Spec.Sink
	if sinkObjRef.Namespace == "" {
		sinkObjRef.Namespace = cronjob.Namespace
	}

	cronjobDesc := fmt.Sprintf("%s/%s,%s", cronjob.Namespace, cronjob.Name, cronjob.GroupVersionKind().String())
	sinkURI, err := r.sinkReconciler.GetSinkURI(sinkObjRef, cronjob, cronjobDesc)
	if err != nil {
		cronjob.Status.MarkNoSink("NotFound", "")
		return fmt.Errorf("getting sink URI: %v", err)
	}
	cronjob.Status.MarkSink(sinkURI)

	_, err = cron.ParseStandard(cronjob.Spec.Schedule)
	if err != nil {
		cronjob.Status.MarkInvalidSchedule("Invalid", "")
		return fmt.Errorf("invalid schedule: %v", err)
	}
	cronjob.Status.MarkSchedule()

	ra, err := r.createReceiveAdapter(ctx, cronjob, sinkURI)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return fmt.Errorf("creating receive adapter: %v", err)
	}
	cronjob.Status.PropagateDeploymentAvailability(ra)

	_, err = r.reconcileEventType(ctx, cronjob)
	if err != nil {
		cronjob.Status.MarkNoEventType("EventTypeReconcileFailed", "")
		return fmt.Errorf("reconciling event types: %v", err)
	}
	cronjob.Status.MarkEventType()

	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.CronJobSource, sinkURI string) (*appsv1.Deployment, error) {
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

	adapterArgs := resources.ReceiveAdapterArgs{
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
		r.Recorder.Eventf(src, corev1.EventTypeNormal, cronJobSourceDeploymentCreated, "%s", msg)
		return ra, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by CronJobSource %q", ra.Name, src.Name)
	} else if podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
			return ra, err
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, cronJobSourceDeploymentUpdated, "Deployment %q updated", ra.Name)
		return ra, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
}

func checkResourcesStatus(src *v1alpha1.CronJobSource) error {
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

func (r *Reconciler) reconcileEventType(ctx context.Context, src *v1alpha1.CronJobSource) (*eventingv1alpha1.EventType, error) {
	expected := resources.MakeEventType(src)
	current, err := r.eventTypeLister.EventTypes(src.Namespace).Get(expected.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing event type", zap.Error(err))
		return nil, fmt.Errorf("getting event types: %v", err)
	}

	// Only create EventTypes for Broker sinks. But if there is an EventType and the src has a non-Broker sink
	// (possibly because it was updated), then we need to delete it.
	if src.Spec.Sink.Kind != "Broker" {
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

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.CronJobSource) (*v1alpha1.CronJobSource, error) {
	cronjob, err := r.cronjobLister.CronJobSources(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(cronjob.Status, desired.Status) {
		return cronjob, nil
	}

	becomesReady := desired.Status.IsReady() && !cronjob.Status.IsReady()

	// Don't modify the informers copy.
	existing := cronjob.DeepCopy()
	existing.Status = desired.Status

	cj, err := r.EventingClientSet.SourcesV1alpha1().CronJobSources(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(cj.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Info("CronJobSource became ready after", zap.Duration("duration", duration))
		r.Recorder.Event(cronjob, corev1.EventTypeNormal, cronJobReadinessChanged, fmt.Sprintf("CronJobSource %q became ready", cronjob.Name))
		if recorderErr := r.StatsReporter.ReportReady("CronJobSource", cronjob.Namespace, cronjob.Name, duration); recorderErr != nil {
			logging.FromContext(ctx).Error("Failed to record ready for CronJobSource", zap.Error(recorderErr))
		}
	}

	return cj, err
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
