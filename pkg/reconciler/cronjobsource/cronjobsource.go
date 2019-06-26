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
	"os"
	"reflect"
	"sync"
	"time"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	eventinglisters "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/sources/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/cronjobsource/resources"
	"knative.dev/pkg/controller"
	"github.com/robfig/cron"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	cronjobReconciled         = "CronJobSourceReconciled"
	cronJobReadinessChanged   = "CronJobSourceReadinessChanged"
	cronjobUpdateStatusFailed = "CronJobSourceUpdateStatusFailed"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raImageEnvVar = "CRONJOB_RA_IMAGE"
)

type Reconciler struct {
	*reconciler.Base

	receiveAdapterImage string
	once                sync.Once

	// listers index properties about resources
	cronjobLister    listers.CronJobSourceLister
	deploymentLister appsv1listers.DeploymentLister
	eventTypeLister  eventinglisters.EventTypeLister

	sinkReconciler *duck.SinkReconciler
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
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the CronJobSource resource with this namespace/name
	original, err := r.cronjobLister.CronJobSources(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("CronJobSource key in work queue no longer exists", zap.Any("key", key))
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
		r.Recorder.Eventf(cronjob, corev1.EventTypeNormal, cronjobReconciled, `CronJobSource reconciled: "%s/%s"`, cronjob.Namespace, cronjob.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, cronjob.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the CronJobSource", zap.Error(err))
		r.Recorder.Eventf(cronjob, corev1.EventTypeWarning, cronjobUpdateStatusFailed, "Failed to update CronJobSource's status: %v", err)
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

	cronjob.Status.InitializeConditions()

	_, err := cron.ParseStandard(cronjob.Spec.Schedule)
	if err != nil {
		cronjob.Status.MarkInvalidSchedule("Invalid", "")
		return err
	}
	cronjob.Status.MarkSchedule()

	if cronjob.Spec.Sink == nil {
		cronjob.Status.MarkNoSink("Missing", "Sink missing from spec")
		return fmt.Errorf("Sink missing from spec")
	}

	sinkObjRef := cronjob.Spec.Sink
	if sinkObjRef.Namespace == "" {
		sinkObjRef.Namespace = cronjob.Namespace
	}

	cronjobDesc := cronjob.Namespace + "/" + cronjob.Name + ", " + cronjob.GroupVersionKind().String()
	sinkURI, err := r.sinkReconciler.GetSinkURI(sinkObjRef, cronjob, cronjobDesc)
	if err != nil {
		cronjob.Status.MarkNoSink("NotFound", "")
		return err
	}
	cronjob.Status.MarkSink(sinkURI)

	_, err = r.createReceiveAdapter(ctx, cronjob, sinkURI)
	if err != nil {
		r.Logger.Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	cronjob.Status.MarkDeployed()

	_, err = r.reconcileEventType(ctx, cronjob)
	if err != nil {
		cronjob.Status.MarkNoEventType("EventTypeReconcileFailed", "")
		return err
	}
	cronjob.Status.MarkEventType()

	return nil
}

func (r *Reconciler) getReceiveAdapterImage() string {
	if r.receiveAdapterImage == "" {
		r.once.Do(func() {
			raImage, defined := os.LookupEnv(raImageEnvVar)
			if !defined {
				panic(fmt.Errorf("required environment variable %q not defined", raImageEnvVar))
			}
			r.receiveAdapterImage = raImage
		})
	}
	return r.receiveAdapterImage
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
				src.Status.MarkResourcesIncorrect("Incorrect Resource", "%s: %s, Error: %s", rsrc.key, rsrc.field, err)
				return err
			}
		}
	}
	src.Status.MarkResourcesCorrect()
	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.CronJobSource, sinkURI string) (*appsv1.Deployment, error) {

	if err := checkResourcesStatus(src); err != nil {
		return nil, err
	}

	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	adapterArgs := resources.ReceiveAdapterArgs{
		Image:   r.getReceiveAdapterImage(),
		Source:  src,
		Labels:  resources.Labels(src.Name),
		SinkURI: sinkURI,
	}
	expected := resources.MakeReceiveAdapter(&adapterArgs)
	if ra != nil {
		if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
			ra.Spec.Template.Spec = expected.Spec.Template.Spec
			if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
				return ra, err
			}
			logging.FromContext(ctx).Info("Receive Adapter updated.", zap.Any("receiveAdapter", ra))
		} else {
			logging.FromContext(ctx).Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		}
		return ra, nil
	}

	if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected); err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("Receive Adapter created.", zap.Any("receiveAdapter", expected))
	return ra, err
}

func (r *Reconciler) podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
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

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.CronJobSource) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
	})
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list cronjobs: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) reconcileEventType(ctx context.Context, src *v1alpha1.CronJobSource) (*eventingv1alpha1.EventType, error) {
	current, err := r.getEventType(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing event type", zap.Error(err))
		return nil, err
	}

	// Only create EventTypes for Broker sinks. But if there is an EventType and the src has a non-Broker sink
	// (possibly because it was updated), then we need to delete it.
	if src.Spec.Sink.Kind != "Broker" {
		if current != nil {
			if err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Delete(current.Name, &metav1.DeleteOptions{}); err != nil {
				logging.FromContext(ctx).Error("Error deleting existing event type", zap.Any("eventType", current))
				return nil, err
			}
		}
		// No current and no error.
		return nil, nil
	}

	expected := resources.MakeEventType(src)
	if current != nil {
		if !equality.Semantic.DeepEqual(expected.Spec, current.Spec) {
			// As is immutable, delete it and create it again.
			if err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Delete(current.Name, &metav1.DeleteOptions{}); err != nil {
				logging.FromContext(ctx).Error("Error deleting existing event type", zap.Any("eventType", current))
				return nil, err
			}
			if current, err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Create(expected); err != nil {
				logging.FromContext(ctx).Error("Error creating event type", zap.Any("eventType", current))
				return nil, err
			}
		}
		return current, nil
	}
	if current, err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Create(expected); err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("EventType created", zap.Any("eventType", current))
	return current, err
}

func (r *Reconciler) getEventType(ctx context.Context, src *v1alpha1.CronJobSource) (*eventingv1alpha1.EventType, error) {
	etl, err := r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
	})
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list event types: %v", zap.Error(err))
		return nil, err
	}
	for _, et := range etl.Items {
		if metav1.IsControlledBy(&et, src) {
			return &et, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) getLabelSelector(src *v1alpha1.CronJobSource) labels.Selector {
	return labels.SelectorFromSet(resources.Labels(src.Name))
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
		r.Logger.Infof("CronJobSource %q became ready after %v", cronjob.Name, duration)
		r.Recorder.Event(cronjob, corev1.EventTypeNormal, cronJobReadinessChanged, fmt.Sprintf("CronJobSource %q became ready", cronjob.Name))
		if err := r.StatsReporter.ReportReady("CronJobSource", cronjob.Namespace, cronjob.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for CronJobSource, %v", err)
		}
	}

	return cj, err
}
