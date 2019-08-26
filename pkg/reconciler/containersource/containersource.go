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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/zap"
	status "knative.dev/eventing/pkg/apis/duck"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/containersource/resources"
	"knative.dev/pkg/controller"
)

const (
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
		Template:           source.Spec.Template,
		Image:              source.Spec.DeprecatedImage,
		Args:               source.Spec.DeprecatedArgs,
		Env:                source.Spec.DeprecatedEnv,
		ServiceAccountName: source.Spec.DeprecatedServiceAccountName,
		Annotations:        annotations,
		Labels:             labels,
	}

	err := r.setSinkURIArg(ctx, source, &args)
	if err != nil {
		r.Recorder.Eventf(source, corev1.EventTypeWarning, "SetSinkURIFailed", "Failed to set Sink URI: %v", err)
		return err
	}

	ra, err := r.reconcileReceiveAdapter(ctx, source, args)
	if err != nil {
		return fmt.Errorf("reconciling receive adapter: %v", err)
	}

	if status.DeploymentIsAvailable(&ra.Status, false) {
		source.Status.MarkDeployed()
		r.Recorder.Eventf(source, corev1.EventTypeNormal, "DeploymentReady", "Deployment %q has %d ready replicas", ra.Name, ra.Status.ReadyReplicas)
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
		source.Status.MarkSink(uri)
		return nil
	}

	if source.Spec.Sink == nil {
		source.Status.MarkNoSink("Missing", "Sink missing from spec")
		return errors.New("sink missing from spec")
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
	var args []string

	if source.Spec.Template != nil {
		for _, c := range source.Spec.Template.Spec.Containers {
			args = append(args, c.Args...)
		}
	}

	args = append(args, source.Spec.DeprecatedArgs...)

	for _, a := range args {
		if strings.HasPrefix(a, "--sink=") {
			return strings.Replace(a, "--sink=", "", -1), true
		}
	}

	return "", false
}

func (r *Reconciler) reconcileReceiveAdapter(ctx context.Context, src *v1alpha1.ContainerSource, args resources.ContainerArguments) (*appsv1.Deployment, error) {
	expected := resources.MakeDeployment(args)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
		if err != nil {
			r.markNotDeployedRecordEvent(src, corev1.EventTypeWarning, "DeploymentCreateFailed", "Could not create deployment: %v", err)
			return nil, fmt.Errorf("creating new deployment: %v", err)
		}
		r.markDeployingAndRecordEvent(src, corev1.EventTypeNormal, "DeploymentCreated", "Created deployment %q", ra.Name)
		return ra, nil
	} else if err != nil {
		r.markDeployingAndRecordEvent(src, corev1.EventTypeWarning, "DeploymentGetFailed", "Error getting deployment: %v", err)
		return nil, fmt.Errorf("getting deployment: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		r.markDeployingAndRecordEvent(src, corev1.EventTypeWarning, "DeploymentNotOwned", "Deployment %q is not owned by this ContainerSource", ra.Name)
		return nil, fmt.Errorf("deployment %q is not owned by ContainerSource %q", ra.Name, src.Name)
	} else if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra)
		if err != nil {
			return ra, fmt.Errorf("updating deployment: %v", err)
		}
		return ra, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
}

func (r *Reconciler) podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
	// Since the Deployment spec has fields defaulted by the webhook, it won't
	// be equal to expected. Use DeepDerivative to compare only the fields that
	// are set in newPodSpec.
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
		if reportErr := r.StatsReporter.ReportReady("ContainerSource", source.Namespace, source.Name, duration); reportErr != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for ContainerSource, %v", reportErr)
		}
	}

	return cj, err
}
