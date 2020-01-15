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

package legacyapiserversource

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	pkgLogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/resolver"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "knative.dev/eventing/pkg/legacyclient/listers/legacysources/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/legacyapiserversource/resources"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	apiserversourceReconciled         = "ApiServerSourceReconciled"
	apiServerSourceReadinessChanged   = "ApiServerSourceReadinessChanged"
	apiserversourceUpdateStatusFailed = "ApiServerSourceUpdateStatusFailed"
	apiserversourceDeploymentCreated  = "ApiServerSourceDeploymentCreated"
	apiserversourceDeploymentUpdated  = "ApiServerSourceDeploymentUpdated"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raImageEnvVar = "APISERVER_RA_IMAGE"

	component = "apiserversource"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
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

	// listers index properties about resources
	apiserversourceLister    listers.ApiServerSourceLister
	deploymentLister         appsv1listers.DeploymentLister
	eventTypeLister          eventinglisters.EventTypeLister
	roleLister               rbacv1listers.RoleLister
	roleBindingLister        rbacv1listers.RoleBindingLister
	clusterRoleLister        rbacv1listers.ClusterRoleLister
	clusterRoleBindingLister rbacv1listers.ClusterRoleBindingLister
	serviceAccountLister     corev1listers.ServiceAccountLister

	source         string
	sinkResolver   *resolver.URIResolver
	loggingContext context.Context
	loggingConfig  *pkgLogging.Config
	metricsConfig  *metrics.ExporterOptions
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ApiServerSource
// resource with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the ApiServerSource resource with this namespace/name
	original, err := r.apiserversourceLister.ApiServerSources(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("ApiServerSource key in work queue no longer exists")
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
	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
	// 3. Create the EventType that it can emit.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
	source.Status.ObservedGeneration = source.Generation

	source.Status.InitializeConditions()

	source.MarkDeprecated(&source.Status.Status, "ApiServerSourceDeprecated", "apiserversources.sources.eventing.knative.dev are deprecated and will be removed in the future. Use apiserversources.sources.knative.dev instead.")

	if source.Spec.Sink == nil {
		source.Status.MarkNoSink("SinkMissing", "")
		return fmt.Errorf("spec.sink missing")
	}

	dest := source.Spec.Sink.DeepCopy()
	if dest.Ref != nil {
		// To call URIFromDestination(), dest.Ref must have a Namespace. If there is
		// no Namespace defined in dest.Ref, we will use the Namespace of the source
		// as the Namespace of dest.Ref.
		if dest.Ref.Namespace == "" {
			//TODO how does this work with deprecated fields
			dest.Ref.Namespace = source.GetNamespace()
		}
	} else if dest.DeprecatedName != "" && dest.DeprecatedNamespace == "" {
		// If Ref is nil and the deprecated ref is present, we need to check for
		// DeprecatedNamespace. This can be removed when DeprecatedNamespace is
		// removed.
		dest.DeprecatedNamespace = source.GetNamespace()
	}

	sinkURI, err := r.sinkResolver.URIFromDestination(*dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return fmt.Errorf("getting sink URI: %v", err)
	}
	if source.Spec.Sink.DeprecatedAPIVersion != "" &&
		source.Spec.Sink.DeprecatedKind != "" &&
		source.Spec.Sink.DeprecatedName != "" {
		source.Status.MarkSinkWarnRefDeprecated(sinkURI)
	} else {
		source.Status.MarkSink(sinkURI)
	}

	err = r.runAccessCheck(source)
	if err != nil {
		logging.FromContext(ctx).Error("Not enough permission", zap.Error(err))
		return err
	}

	ra, err := r.createReceiveAdapter(ctx, source, sinkURI)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	source.Status.PropagateDeploymentAvailability(ra)

	err = r.reconcileEventTypes(ctx, source)
	if err != nil {
		source.Status.MarkNoEventTypes("EventTypesReconcileFailed", "")
		return fmt.Errorf("reconciling event types: %v", err)
	}
	source.Status.MarkEventTypes()

	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.ApiServerSource, sinkURI string) (*appsv1.Deployment, error) {
	// TODO: missing.
	// if err := checkResourcesStatus(src); err != nil {
	// 	return nil, err
	// }

	loggingConfig, err := pkgLogging.LoggingConfigToJson(r.loggingConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting logging config to json", zap.Any("receiveAdapter", err))
	}

	metricsConfig, err := metrics.MetricsOptionsToJson(r.metricsConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting metrics config to json", zap.Any("receiveAdapter", err))
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
		r.Recorder.Eventf(src, corev1.EventTypeNormal, apiserversourceDeploymentCreated, "%s", msg)
		return ra, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by ApiServerSource %q", ra.Name, src.Name)
	} else if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
			return ra, err
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, apiserversourceDeploymentUpdated, "Deployment %q updated", ra.Name)
		return ra, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
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

func (r *Reconciler) reconcileEventTypes(ctx context.Context, src *v1alpha1.ApiServerSource) error {
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

	return nil
}

func (r *Reconciler) getEventTypes(ctx context.Context, src *v1alpha1.ApiServerSource) ([]eventingv1alpha1.EventType, error) {
	etl, err := r.eventTypeLister.EventTypes(src.Namespace).List(r.getLabelSelector(src))
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list event types: %v", zap.Error(err))
		return nil, err
	}
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	for _, et := range etl {
		if metav1.IsControlledBy(et, src) {
			eventTypes = append(eventTypes, *et)
		}
	}
	return eventTypes, nil
}

func (r *Reconciler) makeEventTypes(src *v1alpha1.ApiServerSource) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)

	// Only create EventTypes for Broker sinks.
	// We add this check here in case the APIServerSource was changed from Broker to non-Broker sink.
	// If so, we need to delete the existing ones, thus we return empty expected.
	if ref := src.Spec.Sink.GetRef(); ref == nil || ref.Kind != "Broker" {
		return eventTypes, nil
	}

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

	cj, err := r.LegacyClientSet.SourcesV1alpha1().ApiServerSources(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(cj.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Info("ApiServerSource became ready after", zap.Duration("duration", duration))
		r.Recorder.Event(apiserversource, corev1.EventTypeNormal, apiServerSourceReadinessChanged, fmt.Sprintf("ApiServerSource %q became ready", apiserversource.Name))
		if err := r.StatsReporter.ReportReady("ApiServerSource", apiserversource.Namespace, apiserversource.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for ApiServerSource, %v", err)
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

func (r *Reconciler) runAccessCheck(src *v1alpha1.ApiServerSource) error {
	if src.Spec.Resources == nil || len(src.Spec.Resources) == 0 {
		src.Status.MarkSufficientPermissions()
		return nil
	}

	user := "system:serviceaccount:" + src.Namespace + ":"
	if src.Spec.ServiceAccountName == "" {
		user += "default"
	} else {
		user += src.Spec.ServiceAccountName
	}

	verbs := []string{"get", "list", "watch"}
	resources := src.Spec.Resources
	lastReason := ""

	// Collect all missing permissions.
	missing := ""
	sep := ""

	for _, res := range resources {
		gv, err := schema.ParseGroupVersion(res.APIVersion)
		if err != nil { // shouldn't happened after #2134 is fixed
			return err
		}
		gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: res.Kind, Group: gv.Group, Version: gv.Version})
		missingVerbs := ""
		sep1 := ""
		for _, verb := range verbs {
			sar := &authorizationv1.SubjectAccessReview{
				Spec: authorizationv1.SubjectAccessReviewSpec{
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Namespace: src.Namespace,
						Verb:      verb,
						Group:     gv.Group,
						Resource:  gvr.Resource,
					},
					User: user,
				},
			}

			response, err := r.KubeClientSet.AuthorizationV1().SubjectAccessReviews().Create(sar)
			if err != nil {
				return err
			}

			if !response.Status.Allowed {
				missingVerbs += sep1 + verb
				sep1 = ", "
			}
		}
		if missingVerbs != "" {
			missing += sep + missingVerbs + ` resource "` + gvr.Resource + `" in API group "` + gv.Group + `"`
			sep = ", "
		}
	}
	if missing == "" {
		src.Status.MarkSufficientPermissions()
		return nil
	}

	src.Status.MarkNoSufficientPermissions(lastReason, "User %s cannot %s", user, missing)
	return fmt.Errorf("Insufficient permission: user %s cannot %s", user, missing)

}
