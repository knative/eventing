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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	sourceinformers "github.com/knative/eventing/pkg/client/informers/externalversions/sources/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/sources/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/cronjobsource/resources"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"github.com/robfig/cron"
	"go.uber.org/zap"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "CronJobSources"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cronjob-source-controller"

	// Name of the corev1.Events emitted from the reconciliation process
	cronjobReconciled         = "CronJobSourceReconciled"
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
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	cronjobsourceInformer sourceinformers.CronJobSourceInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
) *controller.Impl {
	r := &Reconciler{
		Base:             reconciler.NewBase(opt, controllerAgentName),
		cronjobLister:    cronjobsourceInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.Logger.Info("Setting up event handlers")
	cronjobsourceInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("CronJobSource")),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

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

	cronjob.Status.InitializeConditions()

	_, err := cron.ParseStandard(cronjob.Spec.Schedule)
	if err != nil {
		cronjob.Status.MarkInvalidSchedule("Invalid", "")
		return err
	}
	cronjob.Status.MarkSchedule()
	sinkURI, err := duck.GetSinkURI(ctx, r.DynamicClientSet, cronjob.Spec.Sink, cronjob.Namespace)
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

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.CronJobSource, sinkURI string) (*appsv1.Deployment, error) {
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
			logging.FromContext(ctx).Desugar().Info("Receive Adapter updated.", zap.Any("receiveAdapter", ra))
		} else {
			logging.FromContext(ctx).Desugar().Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		}
		return ra, nil
	}

	if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected); err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Any("receiveAdapter", expected))
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
		logging.FromContext(ctx).Desugar().Error("Unable to list cronjobs: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
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
		//r.StatsReporter.ReportServiceReady(subscription.Namespace, subscription.Name, duration) // TODO: stats
	}

	return cj, err
}
