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

package apiserversource

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	apiserversourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/apiserversource"
	listers "knative.dev/eventing/pkg/client/listers/sources/v1alpha2"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/eventing/pkg/utils"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	apiserversourceDeploymentCreated = "ApiServerSourceDeploymentCreated"
	apiserversourceDeploymentUpdated = "ApiServerSourceDeploymentUpdated"
	apiserversourceDeploymentDeleted = "ApiServerSourceDeploymentDeleted"

	component = "apiserversource"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason ApiServerSourceReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "ApiServerSourceReconciled", "ApiServerSource reconciled: \"%s/%s\"", namespace, name)
}

func newWarningSinkNotFound(sink *duckv1.Destination) pkgreconciler.Event {
	b, _ := json.Marshal(sink)
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "SinkNotFound", "Sink not found: %s", string(b))
}

// Reconciler reconciles a ApiServerSource object
type Reconciler struct {
	kubeClientSet kubernetes.Interface

	receiveAdapterImage string

	// listers index properties about resources
	apiserversourceLister listers.ApiServerSourceLister

	ceSource       string
	sinkResolver   *resolver.URIResolver
	loggingContext context.Context

	configs reconcilersource.ConfigAccessor
}

var _ apiserversourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha2.ApiServerSource) pkgreconciler.Event {
	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
	// 3. Create the EventType that it can emit.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
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

	err = r.runAccessCheck(source)
	if err != nil {
		logging.FromContext(ctx).Error("Not enough permission", zap.Error(err))
		return err
	}

	ra, err := r.createReceiveAdapter(ctx, source, sinkURI.String())
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	source.Status.PropagateDeploymentAvailability(ra)

	source.Status.CloudEventAttributes = r.createCloudEventAttributes()

	return newReconciledNormal(source.Namespace, source.Name)
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha2.ApiServerSource, sinkURI string) (*appsv1.Deployment, error) {
	// TODO: missing.
	// if err := checkResourcesStatus(src); err != nil {
	// 	return nil, err
	// }

	adapterArgs := resources.ReceiveAdapterArgs{
		Image:   r.receiveAdapterImage,
		Source:  src,
		Labels:  resources.Labels(src.Name),
		SinkURI: sinkURI,
		Configs: r.configs,
	}
	expected, err := resources.MakeReceiveAdapter(&adapterArgs)
	if err != nil {
		return nil, err
	}

	ra, err := r.kubeClientSet.AppsV1().Deployments(src.Namespace).Get(expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Issue #2842: Adater deployment name uses kmeta.ChildName. If a deployment by the previous name pattern is found, it should
		// be deleted. This might cause temporary downtime.
		if deprecatedName := utils.GenerateFixedName(adapterArgs.Source, fmt.Sprintf("apiserversource-%s", adapterArgs.Source.Name)); deprecatedName != expected.Name {
			if err := r.kubeClientSet.AppsV1().Deployments(src.Namespace).Delete(deprecatedName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("error deleting deprecated named deployment: %v", err)
			}
			controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal, apiserversourceDeploymentDeleted, "Deprecated deployment removed: \"%s/%s\"", src.Namespace, deprecatedName)
		}

		ra, err = r.kubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
		msg := "Deployment created"
		if err != nil {
			msg = fmt.Sprintf("Deployment created, error: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal, apiserversourceDeploymentCreated, "%s", msg)
		return ra, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by ApiServerSource %q", ra.Name, src.Name)
	} else if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.kubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
			return ra, err
		}
		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal, apiserversourceDeploymentUpdated, "Deployment %q updated", ra.Name)
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

func (r *Reconciler) runAccessCheck(src *v1alpha2.ApiServerSource) error {
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
	lastReason := ""

	// Collect all missing permissions.
	missing := ""
	sep := ""

	for _, res := range src.Spec.Resources {
		gv, err := schema.ParseGroupVersion(res.APIVersion)
		if err != nil {
			return err
		}
		gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: res.Kind, Group: gv.Group, Version: gv.Version}) // TODO: Test for nil Kind.
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

			response, err := r.kubeClientSet.AuthorizationV1().SubjectAccessReviews().Create(sar)
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

func (r *Reconciler) createCloudEventAttributes() []duckv1.CloudEventAttributes {
	ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(v1alpha2.ApiServerSourceEventTypes))
	for _, apiServerSourceType := range v1alpha2.ApiServerSourceEventTypes {
		ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
			Type:   apiServerSourceType,
			Source: r.ceSource,
		})
	}
	return ceAttributes
}
