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

package containersource

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	sourceinformers "github.com/knative/eventing/pkg/client/informers/externalversions/sources/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/sources/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/containersource/resources"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "ContainerSources"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "container-source-controller"

	// Name of the corev1.Events emitted from the reconciliation process
	sourceReconciled         = "ContainerSourceReconciled"
	sourceReadinessChanged   = "ContainerSourceReadinessChanged"
	sourceUpdateStatusFailed = "ContainerSourceUpdateStatusFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	containerSourceLister listers.ContainerSourceLister
	deploymentLister      appsv1listers.DeploymentLister

	sinkReconciler *duck.SinkReconciler
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	containerSourceInformer sourceinformers.ContainerSourceInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
) *controller.Impl {
	r := &Reconciler{
		Base:                  reconciler.NewBase(opt, controllerAgentName),
		containerSourceLister: containerSourceInformer.Lister(),
		deploymentLister:      deploymentInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)
	r.sinkReconciler = duck.NewSinkReconciler(opt, impl.EnqueueKey)

	r.Logger.Info("Setting up event handlers")
	containerSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ContainerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
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
	original, err := r.containerSourceLister.ContainerSources(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("ContainerSource key in work queue no longer exists", zap.Any("key", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	source := original.DeepCopy()

	// Reconcile this copy of the ContainerSource and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Warn("Error reconciling ContainerSource", zap.Error(err))
	} else {
		logging.FromContext(ctx).Debug("ContainerSource reconciled")
		r.Recorder.Eventf(source, corev1.EventTypeNormal, sourceReconciled, `ContainerSource reconciled: "%s/%s"`, source.Namespace, source.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, source.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the ContainerSource", zap.Error(err))
		r.Recorder.Eventf(source, corev1.EventTypeWarning, sourceUpdateStatusFailed, "Failed to update ContainerSource's status: %v", err)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return err
}

func (r *Reconciler) reconcile(ctx context.Context, source *v1alpha1.ContainerSource) error {
	// No need to reconcile if the source has been marked for deletion.
	if source.DeletionTimestamp != nil {
		return nil
	}

	source.Status.InitializeConditions()

	annotations := make(map[string]string)
	// Then wire through any annotations / labels from the Source
	if source.ObjectMeta.Annotations != nil {
		for k, v := range source.ObjectMeta.Annotations {
			annotations[k] = v
		}
	}
	labels := make(map[string]string)
	if source.ObjectMeta.Labels != nil {
		for k, v := range source.ObjectMeta.Labels {
			labels[k] = v
		}
	}

	args := resources.ContainerArguments{
		Source:             source,
		Name:               source.Name,
		Namespace:          source.Namespace,
		Image:              source.Spec.Image,
		Args:               source.Spec.Args,
		Env:                source.Spec.Env,
		ServiceAccountName: source.Spec.ServiceAccountName,
		Annotations:        annotations,
		Labels:             labels,
	}

	err := r.setSinkURIArg(ctx, source, &args)
	if err != nil {
		r.Recorder.Eventf(source, corev1.EventTypeWarning, "SetSinkURIFailed", "Failed to set Sink URI: %v", err)
		return err
	}

	deploy, err := r.getDeployment(ctx, source)
	if err != nil {
		if errors.IsNotFound(err) {
			deploy, err = r.createDeployment(ctx, source, nil, args)
			if err != nil {
				r.markNotDeployedRecordEvent(source, corev1.EventTypeWarning, "DeploymentCreateFailed", "Could not create deployment: %v", err)
				return err
			}
			r.markDeployingAndRecordEvent(source, corev1.EventTypeNormal, "DeploymentCreated", "Created deployment %q", deploy.Name)
			// Since the Deployment has just been created, there's nothing more
			// to do until it gets a status. This ContainerSource will be reconciled
			// again when the Deployment is updated.
			return nil
		}
		// Something unexpected happened getting the deployment.
		r.markDeployingAndRecordEvent(source, corev1.EventTypeWarning, "DeploymentGetFailed", "Error getting deployment: %v", err)
		return err
	}

	// Update Deployment spec if it's changed
	expected := resources.MakeDeployment(args)
	// Since the Deployment spec has fields defaulted by the webhook, it won't
	// be equal to expected. Use DeepDerivative to compare only the fields that
	// are set in expected.
	if !equality.Semantic.DeepDerivative(expected.Spec, deploy.Spec) {
		deploy.Spec = expected.Spec
		deploy, err := r.KubeClientSet.AppsV1().Deployments(deploy.Namespace).Update(deploy)
		if err != nil {
			r.markDeployingAndRecordEvent(source, corev1.EventTypeWarning, "DeploymentUpdateFailed", "Failed to update deployment %q: %v", deploy.Name, err)
		} else {
			r.markDeployingAndRecordEvent(source, corev1.EventTypeNormal, "DeploymentUpdated", "Updated deployment %q", deploy.Name)
		}
		// Return after this update or error and reconcile again
		return err
	}

	// Update source status
	if deploy.Status.ReadyReplicas > 0 && !source.Status.IsDeployed() {
		source.Status.MarkDeployed()
		r.Recorder.Eventf(source, corev1.EventTypeNormal, "DeploymentReady", "Deployment %q has %d ready replicas", deploy.Name, deploy.Status.ReadyReplicas)
	}

	return nil
}

// setSinkURIArg attempts to get the sink URI from the sink reference and
// set it in the source status. On failure, the source's Sink condition is
// updated to reflect the error.
// If an error is returned from this function, the caller should also record
// an Event containing the error string.
func (r *Reconciler) setSinkURIArg(ctx context.Context, source *v1alpha1.ContainerSource, args *resources.ContainerArguments) error {
	if uri, ok := sinkArg(source); ok {
		args.SinkInArgs = true
		source.Status.MarkSink(uri)
		return nil
	}

	if source.Spec.Sink == nil {
		source.Status.MarkNoSink("Missing", "Sink missing from spec")
		return fmt.Errorf("Sink missing from spec")
	}

	sinkObjRef := source.Spec.Sink
	if sinkObjRef.Namespace == "" {
		sinkObjRef.Namespace = source.Namespace
	}

	sourceDesc := source.Namespace + "/" + source.Name + ", " + source.GroupVersionKind().String()
	uri, err := r.sinkReconciler.GetSinkURI(source.Spec.Sink, source, sourceDesc)
	if err != nil {
		source.Status.MarkNoSink("NotFound", `Couldn't get Sink URI from "%s/%s": %v"`, source.Spec.Sink.Namespace, source.Spec.Sink.Name, err)
		return err
	}
	source.Status.MarkSink(uri)
	args.Sink = uri

	return nil
}

func sinkArg(source *v1alpha1.ContainerSource) (string, bool) {
	for _, a := range source.Spec.Args {
		if strings.HasPrefix(a, "--sink=") {
			return strings.Replace(a, "--sink=", "", -1), true
		}
	}
	return "", false
}

func (r *Reconciler) getDeployment(ctx context.Context, source *v1alpha1.ContainerSource) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(source.Namespace).List(metav1.ListOptions{})
	if err != nil {
		r.Logger.Errorf("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, c := range dl.Items {
		if metav1.IsControlledBy(&c, source) {
			return &c, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) createDeployment(ctx context.Context, source *v1alpha1.ContainerSource, org *appsv1.Deployment, args resources.ContainerArguments) (*appsv1.Deployment, error) {
	deployment := resources.MakeDeployment(args)
	return r.KubeClientSet.AppsV1().Deployments(source.Namespace).Create(deployment)
}

func (r *Reconciler) markDeployingAndRecordEvent(source *v1alpha1.ContainerSource, evType string, reason string, messageFmt string, args ...interface{}) {
	r.Recorder.Eventf(source, evType, reason, messageFmt, args...)
	source.Status.MarkDeploying(reason, messageFmt, args...)
}

func (r *Reconciler) markNotDeployedRecordEvent(source *v1alpha1.ContainerSource, evType string, reason string, messageFmt string, args ...interface{}) {
	r.Recorder.Eventf(source, evType, reason, messageFmt, args...)
	source.Status.MarkNotDeployed(reason, messageFmt, args...)
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.ContainerSource) (*v1alpha1.ContainerSource, error) {
	source, err := r.containerSourceLister.ContainerSources(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(source.Status, desired.Status) {
		return source, nil
	}

	becomesReady := desired.Status.IsReady() && !source.Status.IsReady()

	// Don't modify the informers copy.
	existing := source.DeepCopy()
	existing.Status = desired.Status

	cj, err := r.EventingClientSet.SourcesV1alpha1().ContainerSources(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(cj.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("ContainerSource %q became ready after %v", source.Name, duration)
		r.Recorder.Event(source, corev1.EventTypeNormal, sourceReadinessChanged, fmt.Sprintf("ContainerSource %q became ready", source.Name))
		if err := r.StatsReporter.ReportReady("ContainerSource", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for ContainerSource, %v", err)
		}
	}

	return cj, err
}
