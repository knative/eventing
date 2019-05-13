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

package apiserversource

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	sourceinformers "github.com/knative/eventing/pkg/client/informers/externalversions/sources/v1alpha1"
	eventinglisters "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/sources/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/apiserversource/resources"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
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
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "ApiServerSources"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "apiserver-source-controller"

	// Name of the corev1.Events emitted from the reconciliation process
	apiserversourceReconciled         = "ApiServerSourceReconciled"
	apiServerSourceReadinessChanged   = "ApiServerSourceReadinessChanged"
	apiserversourceUpdateStatusFailed = "ApiServerSourceUpdateStatusFailed"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raImageEnvVar = "APISERVER_RA_IMAGE"
)

var apiServerEventTypes = []string{
	v1alpha1.ApiServerSourceAddEventType,
	v1alpha1.ApiServerSourceDeleteEventType,
	v1alpha1.ApiServerSourceUpdateEventType,
	v1alpha1.ApiServerSourceAddRefEventType,
	v1alpha1.ApiServerSourceDeleteRefEventType,
	v1alpha1.ApiServerSourceUpdateRefEventType,
}

// Reconciler reconciles a ApiServerSource object
type Reconciler struct {
	*reconciler.Base

	receiveAdapterImage string
	once                sync.Once

	// listers index properties about resources
	apiserversourceLister listers.ApiServerSourceLister
	deploymentLister      appsv1listers.DeploymentLister
	eventTypeLister       eventinglisters.EventTypeLister

	source         string
	sinkReconciler *duck.SinkReconciler
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	apiserversourceInformer sourceinformers.ApiServerSourceInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	eventTypeInformer eventinginformers.EventTypeInformer,
	source string,
) *controller.Impl {
	r := &Reconciler{
		Base:                  reconciler.NewBase(opt, controllerAgentName),
		apiserversourceLister: apiserversourceInformer.Lister(),
		deploymentLister:      deploymentInformer.Lister(),
		source:                source,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.sinkReconciler = duck.NewSinkReconciler(opt, impl.EnqueueKey)

	r.Logger.Info("Setting up event handlers")
	apiserversourceInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ApiServerSource")),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	eventTypeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ApiServerSource")),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ApiServerSource
// resource with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the ApiServerSource resource with this namespace/name
	original, err := r.apiserversourceLister.ApiServerSources(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("ApiServerSource key in work queue no longer exists", zap.Any("key", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	apiserversource := original.DeepCopy()

	// Reconcile this copy of the ApiServerSource and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, apiserversource)
	if err != nil {
		logging.FromContext(ctx).Warn("Error reconciling ApiServerSource", zap.Error(err))
	} else {
		logging.FromContext(ctx).Debug("ApiServerSource reconciled")
		r.Recorder.Eventf(apiserversource, corev1.EventTypeNormal, apiserversourceReconciled, `ApiServerSource reconciled: "%s/%s"`, apiserversource.Namespace, apiserversource.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, apiserversource.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the ApiServerSource", zap.Error(err))
		r.Recorder.Eventf(apiserversource, corev1.EventTypeWarning, apiserversourceUpdateStatusFailed, "Failed to update ApiServerSource's status: %v", err)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return err
}

func (r *Reconciler) reconcile(ctx context.Context, source *v1alpha1.ApiServerSource) error {
	source.Status.InitializeConditions()

	sinkObjRef := source.Spec.Sink
	if sinkObjRef.Namespace == "" {
		sinkObjRef.Namespace = source.Namespace
	}

	sourceDesc := source.Namespace + "/" + source.Name + ", " + source.GroupVersionKind().String()
	sinkURI, err := r.sinkReconciler.GetSinkURI(sinkObjRef, source, sourceDesc)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}
	source.Status.MarkSink(sinkURI)

	_, err = r.createReceiveAdapter(ctx, source, sinkURI)
	if err != nil {
		r.Logger.Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	// Update source status
	source.Status.MarkDeployed()

	// Only create EventType for Broker sinks.
	if source.Spec.Sink.Kind == "Broker" {
		err = r.createEventTypes(ctx, source)
		if err != nil {
			source.Status.MarkNoEventTypes("EventTypesCreateFailed", "")
			return err
		}
		source.Status.MarkEventTypes()
	}
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

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.ApiServerSource, sinkURI string) (*appsv1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	if ra != nil {
		logging.FromContext(ctx).Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		return ra, nil
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

func (r *Reconciler) createEventTypes(ctx context.Context, src *v1alpha1.ApiServerSource) error {
	current, err := r.getEventTypes(ctx, src)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get existing event types", zap.Error(err))
		return err
	}

	expected, err := r.makeEventTypes(src)
	if err != nil {
		return err
	}

	toCreate, toDelete := r.computeDiff(current, expected)

	for _, eventType := range toDelete {
		if err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Delete(eventType.Name, &metav1.DeleteOptions{}); err != nil {
			logging.FromContext(ctx).Error("Error deleting eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	for _, eventType := range toCreate {
		if _, err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Create(&eventType); err != nil {
			logging.FromContext(ctx).Error("Error creating eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	return err
}

func (r *Reconciler) getEventTypes(ctx context.Context, src *v1alpha1.ApiServerSource) ([]eventingv1alpha1.EventType, error) {
	etl, err := r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
	})
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list event types: %v", zap.Error(err))
		return nil, err
	}
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	for _, et := range etl.Items {
		if metav1.IsControlledBy(&et, src) {
			eventTypes = append(eventTypes, et)
		}
	}
	return eventTypes, nil
}

func (r *Reconciler) makeEventTypes(src *v1alpha1.ApiServerSource) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	args := &resources.EventTypeArgs{
		Src:    src,
		Source: r.source,
	}
	for _, apiEventType := range apiServerEventTypes {
		args.Type = apiEventType
		eventType := resources.MakeEventType(args)
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, nil
}

func (r *Reconciler) computeDiff(current []eventingv1alpha1.EventType, expected []eventingv1alpha1.EventType) ([]eventingv1alpha1.EventType, []eventingv1alpha1.EventType) {
	toCreate := make([]eventingv1alpha1.EventType, 0)
	toDelete := make([]eventingv1alpha1.EventType, 0)
	currentMap := asMap(current, keyFromEventType)
	expectedMap := asMap(expected, keyFromEventType)

	// Iterate over the slices instead of the maps for predictable UT expectations.
	for _, e := range expected {
		if c, ok := currentMap[keyFromEventType(&e)]; !ok {
			toCreate = append(toCreate, e)
		} else {
			if !equality.Semantic.DeepEqual(e.Spec, c.Spec) {
				toDelete = append(toDelete, c)
				toCreate = append(toCreate, e)
			}
		}
	}
	// Need to check whether the current EventTypes are not in the expected map. If so, we have to delete them.
	// This could happen if the ApiServerSource CO changes its broker.
	for _, c := range current {
		if _, ok := expectedMap[keyFromEventType(&c)]; !ok {
			toDelete = append(toDelete, c)
		}
	}
	return toCreate, toDelete
}

func asMap(eventTypes []eventingv1alpha1.EventType, keyFunc func(*eventingv1alpha1.EventType) string) map[string]eventingv1alpha1.EventType {
	eventTypesAsMap := make(map[string]eventingv1alpha1.EventType, 0)
	for _, eventType := range eventTypes {
		key := keyFunc(&eventType)
		eventTypesAsMap[key] = eventType
	}
	return eventTypesAsMap
}

func keyFromEventType(eventType *eventingv1alpha1.EventType) string {
	return fmt.Sprintf("%s_%s_%s_%s", eventType.Spec.Type, eventType.Spec.Source, eventType.Spec.Schema, eventType.Spec.Broker)
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

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.ApiServerSource) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
	})
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) getLabelSelector(src *v1alpha1.ApiServerSource) labels.Selector {
	return labels.SelectorFromSet(resources.Labels(src.Name))
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.ApiServerSource) (*v1alpha1.ApiServerSource, error) {
	apiserversource, err := r.apiserversourceLister.ApiServerSources(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(apiserversource.Status, desired.Status) {
		return apiserversource, nil
	}

	becomesReady := desired.Status.IsReady() && !apiserversource.Status.IsReady()

	// Don't modify the informers copy.
	existing := apiserversource.DeepCopy()
	existing.Status = desired.Status

	cj, err := r.EventingClientSet.SourcesV1alpha1().ApiServerSources(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(cj.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("ApiServerSource %q became ready after %v", apiserversource.Name, duration)
		r.Recorder.Event(apiserversource, corev1.EventTypeNormal, apiServerSourceReadinessChanged, fmt.Sprintf("ApiServerSource %q became ready", apiserversource.Name))
		if err := r.StatsReporter.ReportReady("ApiServerSource", apiserversource.Namespace, apiserversource.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for ApiServerSource, %v", err)
		}
	}

	return cj, err
}
