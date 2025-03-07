/*
Copyright 2025 The Knative Authors

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

package eventtransform

import (
	"context"
	"fmt"

	cmapis "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appslister "k8s.io/client-go/listers/apps/v1"
	corelister "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing/pkg/apis/feature"
	cmclient "knative.dev/eventing/pkg/client/certmanager/clientset/versioned"
	cmlisters "knative.dev/eventing/pkg/client/certmanager/listers/certmanager/v1"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/reconciler"

	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sources "knative.dev/eventing/pkg/apis/sources/v1"
	eventingclient "knative.dev/eventing/pkg/client/clientset/versioned"
	sourceslisters "knative.dev/eventing/pkg/client/listers/sources/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

type Reconciler struct {
	k8s      kubernetes.Interface
	client   eventingclient.Interface
	cmClient cmclient.Interface

	jsonataConfigMapLister     corelister.ConfigMapLister
	jsonataDeploymentsLister   appslister.DeploymentLister
	jsonataServiceLister       corelister.ServiceLister
	jsonataEndpointLister      corelister.EndpointsLister
	jsonataSinkBindingLister   sourceslisters.SinkBindingLister
	cmCertificateLister        cmlisters.CertificateLister
	certificatesSecretLister   corelister.SecretLister
	trustBundleConfigMapLister corelister.ConfigMapLister

	configWatcher *reconcilersource.ConfigWatcher
}

func (r *Reconciler) ReconcileKind(ctx context.Context, transform *eventing.EventTransform) reconciler.Event {
	if err := r.reconcileJsonataTransformation(ctx, transform); err != nil {
		return fmt.Errorf("failed to reconcile Jsonata transformation: %w", err)
	}
	return nil
}

func (r *Reconciler) reconcileJsonataTransformation(ctx context.Context, transform *eventing.EventTransform) error {
	logger := logging.FromContext(ctx)

	if transform.Spec.EventTransformations.Jsonata == nil {
		logger.Debug("No Jsonata transformation found")
		return nil
	}

	logger.Debugw("Reconciling Jsonata transformation ConfigMap")
	expressionCm, err := r.reconcileJsonataTransformationConfigMap(ctx, transform)
	if err != nil {
		return fmt.Errorf("failed to reconcile Jsonata transformation deployment: %w", err)
	}

	logger.Debugw("Reconciling Jsonata transformation Service")
	if err := r.reconcileJsonataTransformationService(ctx, transform); err != nil {
		return fmt.Errorf("failed to reconcile Jsonata transformation deployment: %w", err)
	}

	logger.Debugw("Reconciling Jsonata transformation Certificate")
	certificate, err := r.reconcileJsonataTransformationCertificate(ctx, transform)
	if err != nil {
		return fmt.Errorf("failed to reconcile Jsonata transformation deployment: %w", err)
	}

	logger.Debugw("Reconciling Jsonata transformation Deployment")
	if err := r.reconcileJsonataTransformationDeployment(ctx, expressionCm, certificate, transform); err != nil {
		return fmt.Errorf("failed to reconcile Jsonata transformation deployment: %w", err)
	}

	logger.Debugw("Reconciling Jsonata transformation SinkBinding")
	if err := r.reconcileJsonataTransformationSinkBinding(ctx, transform); err != nil {
		return fmt.Errorf("failed to reconcile Jsonata transformation sink binding: %w", err)
	}

	logger.Debugw("Reconciling Jsonata transformation address")
	if err := r.reconcileJsonataTransformationAddress(ctx, transform); err != nil {
		return fmt.Errorf("failed to reconcile Jsonata transformation address: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileJsonataTransformationConfigMap(ctx context.Context, transform *eventing.EventTransform) (*corev1.ConfigMap, error) {
	expected := jsonataExpressionConfigMap(ctx, transform)

	curr, err := r.jsonataConfigMapLister.ConfigMaps(expected.GetNamespace()).Get(expected.GetName())
	if apierrors.IsNotFound(err) {
		return r.createConfigMap(ctx, transform, expected)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	if equality.Semantic.DeepDerivative(expected.Data, curr.Data) &&
		equality.Semantic.DeepDerivative(expected.Labels, curr.Labels) &&
		equality.Semantic.DeepDerivative(expected.Annotations, curr.Annotations) {
		return curr, nil
	}
	expected.ResourceVersion = curr.ResourceVersion
	return r.updateConfigMap(ctx, transform, expected)
}

func (r *Reconciler) reconcileJsonataTransformationService(ctx context.Context, transform *eventing.EventTransform) error {
	expected := jsonataService(ctx, transform)

	curr, err := r.jsonataServiceLister.Services(expected.GetNamespace()).Get(expected.GetName())
	if apierrors.IsNotFound(err) {
		_, err := r.createService(ctx, transform, expected)
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get service %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	if equality.Semantic.DeepDerivative(expected.Spec, curr.Spec) &&
		equality.Semantic.DeepDerivative(expected.Labels, curr.Labels) &&
		equality.Semantic.DeepDerivative(expected.Annotations, curr.Annotations) {
		return nil
	}
	expected.ResourceVersion = curr.ResourceVersion
	_, err = r.updateService(ctx, transform, expected)
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileJsonataTransformationCertificate(ctx context.Context, transform *eventing.EventTransform) (*cmapis.Certificate, error) {
	if f := feature.FromContext(ctx); !f.IsStrictTransportEncryption() && !f.IsPermissiveTransportEncryption() {
		return nil, r.deleteJsonataTransformationCertificate(ctx, transform)
	}
	expected := jsonataCertificate(ctx, transform)

	curr, err := r.cmCertificateLister.Certificates(expected.GetNamespace()).Get(expected.GetName())
	if apierrors.IsNotFound(err) {
		created, err := r.createCertificate(ctx, transform, expected)
		if err != nil {
			return nil, err
		}
		if !transform.Status.PropagateJsonataCertificateStatus(created.Status) {
			// Wait for Certificate to become ready before continuing.
			return nil, controller.NewSkipKey("")
		}
		return created, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	if equality.Semantic.DeepDerivative(expected.Spec, curr.Spec) &&
		equality.Semantic.DeepDerivative(expected.Labels, curr.Labels) &&
		equality.Semantic.DeepDerivative(expected.Annotations, curr.Annotations) {
		if !transform.Status.PropagateJsonataCertificateStatus(curr.Status) {
			// Wait for Certificate to become ready before continuing.
			return nil, controller.NewSkipKey("")
		}
		return curr, nil
	}
	expected.ResourceVersion = curr.ResourceVersion
	updated, err := r.updateCertificate(ctx, transform, expected)
	if err != nil {
		return nil, err
	}
	if !transform.Status.PropagateJsonataCertificateStatus(updated.Status) {
		// Wait for Certificate to become ready before continuing.
		return nil, controller.NewSkipKey("")
	}
	return updated, nil
}

func (r *Reconciler) reconcileJsonataTransformationDeployment(ctx context.Context, expression *corev1.ConfigMap, certificate *cmapis.Certificate, transform *eventing.EventTransform) error {
	withCombinedTrustBundle := false
	if isPresent, _ := eventingtls.CombinedBundlePresent(r.trustBundleConfigMapLister); isPresent {
		withCombinedTrustBundle = true
	}
	expected := jsonataDeployment(ctx, withCombinedTrustBundle, r.configWatcher, expression, certificate, transform)

	curr, err := r.jsonataDeploymentsLister.Deployments(expected.GetNamespace()).Get(expected.GetName())
	if apierrors.IsNotFound(err) {
		created, err := r.createDeployment(ctx, transform, expected)
		if err != nil {
			return err
		}
		if !transform.Status.PropagateJsonataDeploymentStatus(created.Status) {
			// Wait for Deployment to become ready before continuing.
			return controller.NewSkipKey("")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	if equality.Semantic.DeepDerivative(expected.Spec, curr.Spec) &&
		equality.Semantic.DeepDerivative(expected.Labels, curr.Labels) &&
		equality.Semantic.DeepDerivative(expected.Annotations, curr.Annotations) {
		if !transform.Status.PropagateJsonataDeploymentStatus(curr.Status) {
			// Wait for Deployment to become ready before continuing.
			return controller.NewSkipKey("")
		}
		return nil
	}

	if logger := logging.FromContext(ctx); logger.Desugar().Core().Enabled(zapcore.DebugLevel) {
		logger.Infow(
			"Updating deployment "+cmp.Diff(expected.Spec, curr.Spec),
			"diff.spec", cmp.Diff(expected.Spec, curr.Spec),
			"diff.labels", cmp.Diff(expected.Labels, curr.Labels),
			"diff.annotations", cmp.Diff(expected.Annotations, curr.Annotations),
		)
	}

	expected.ResourceVersion = curr.ResourceVersion
	updated, err := r.updateDeployment(ctx, transform, expected)
	if err != nil {
		return err
	}
	if !transform.Status.PropagateJsonataDeploymentStatus(updated.Status) {
		// Wait for Deployment to become ready before continuing.
		return controller.NewSkipKey("")
	}
	return nil
}

func (r *Reconciler) reconcileJsonataTransformationSinkBinding(ctx context.Context, transform *eventing.EventTransform) error {
	if transform.Spec.Sink == nil {
		transform.Status.PropagateJsonataSinkBindingUnset()
		return r.deleteJsonataTransformationSinkBinding(ctx, transform)
	}

	expected := jsonataSinkBinding(ctx, transform)
	curr, err := r.jsonataSinkBindingLister.SinkBindings(expected.GetNamespace()).Get(expected.GetName())
	if apierrors.IsNotFound(err) {
		created, err := r.createSinkBinding(ctx, transform, expected)
		if err != nil {
			return err
		}
		if !transform.Status.PropagateJsonataSinkBindingStatus(created.Status) {
			// Wait for SinkBinding to become ready before continuing.
			return controller.NewSkipKey("")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	if equality.Semantic.DeepDerivative(expected.Spec, curr.Spec) &&
		equality.Semantic.DeepDerivative(expected.Labels, curr.Labels) &&
		equality.Semantic.DeepDerivative(expected.Annotations, curr.Annotations) {
		if !transform.Status.PropagateJsonataSinkBindingStatus(curr.Status) {
			// Wait for SinkBinding to become ready before continuing.
			return controller.NewSkipKey("")
		}
		return nil
	}
	expected.ResourceVersion = curr.ResourceVersion
	updated, err := r.updateSinkBinding(ctx, transform, expected)
	if err != nil {
		return err
	}
	if !transform.Status.PropagateJsonataSinkBindingStatus(updated.Status) {
		// Wait for SinkBinding to become ready before continuing.
		return controller.NewSkipKey("")
	}

	return nil
}

func (r *Reconciler) reconcileJsonataTransformationAddress(ctx context.Context, transform *eventing.EventTransform) error {
	service := jsonataService(ctx, transform)
	endpoint, err := r.jsonataEndpointLister.Endpoints(transform.GetNamespace()).Get(service.GetName())
	if apierrors.IsNotFound(err) {
		transform.Status.MarkWaitingForServiceEndpoints()
		return controller.NewSkipKey("")
	}
	if err != nil {
		return fmt.Errorf("failed to list jsonata endpoints: %w", err)
	}
	if len(endpoint.Subsets) == 0 || len(endpoint.Subsets[0].Ports) == 0 || len(endpoint.Subsets[0].Addresses) == 0 {
		transform.Status.MarkWaitingForServiceEndpoints()
		return controller.NewSkipKey("")
	}

	if f := feature.FromContext(ctx); f.IsStrictTransportEncryption() {
		for _, sub := range endpoint.Subsets {
			for _, p := range sub.Ports {
				if p.Port != 8443 {
					transform.Status.MarkWaitingForServiceEndpoints()
					return controller.NewSkipKey("")
				}
			}
		}
	}

	hostname := network.GetServiceHostname(service.GetName(), service.GetNamespace())

	if feature.FromContext(ctx).IsStrictTransportEncryption() {
		transform.Status.SetAddresses(
			duckv1.Addressable{
				Name:    ptr.String("https"),
				URL:     apis.HTTPS(hostname),
				CACerts: r.jsonataCaCerts(ctx, transform),
			},
		)
	} else if feature.FromContext(ctx).IsPermissiveTransportEncryption() {
		transform.Status.SetAddresses(
			duckv1.Addressable{
				Name:    ptr.String("https"),
				URL:     apis.HTTPS(hostname),
				CACerts: r.jsonataCaCerts(ctx, transform),
			},
			duckv1.Addressable{
				Name: ptr.String("http"),
				URL:  apis.HTTP(hostname),
			},
		)
	} else {
		transform.Status.SetAddresses(duckv1.Addressable{
			Name: ptr.String("http"),
			URL:  apis.HTTP(hostname),
		})
	}

	// TODO: Support Authn/z (control plane and data plane)

	return nil
}

func (r *Reconciler) jsonataCaCerts(_ context.Context, transform *eventing.EventTransform) *string {
	s, err := r.certificatesSecretLister.Secrets(transform.GetNamespace()).Get(jsonataCertificateSecretName(transform))
	if err != nil {
		return nil
	}
	ca, ok := s.Data[eventingtls.SecretCACert]
	if !ok {
		return nil
	}
	if len(ca) == 0 {
		return nil
	}
	return ptr.String(string(ca))
}

func (r *Reconciler) createService(ctx context.Context, transform *eventing.EventTransform, expected corev1.Service) (*corev1.Service, error) {
	created, err := r.k8s.CoreV1().Services(expected.GetNamespace()).Create(ctx, &expected, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonata configmap %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataServiceCreated", expected.GetName())
	return created, nil
}

func (r *Reconciler) updateService(ctx context.Context, transform *eventing.EventTransform, expected corev1.Service) (*corev1.Service, error) {
	updated, err := r.k8s.CoreV1().Services(expected.GetNamespace()).Update(ctx, &expected, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update configmap %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataServiceUpdated", expected.GetName())
	return updated, nil
}

func (r *Reconciler) createDeployment(ctx context.Context, transform *eventing.EventTransform, expected appsv1.Deployment) (*appsv1.Deployment, error) {
	created, err := r.k8s.AppsV1().Deployments(expected.GetNamespace()).Create(ctx, &expected, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonata deployment %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataDeploymentCreated", expected.GetName())
	return created, nil
}

func (r *Reconciler) updateDeployment(ctx context.Context, transform *eventing.EventTransform, expected appsv1.Deployment) (*appsv1.Deployment, error) {
	updated, err := r.k8s.AppsV1().Deployments(expected.GetNamespace()).Update(ctx, &expected, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataDeploymentUpdated", expected.GetName())
	return updated, nil
}

func (r *Reconciler) createConfigMap(ctx context.Context, transform *eventing.EventTransform, expected corev1.ConfigMap) (*corev1.ConfigMap, error) {
	created, err := r.k8s.CoreV1().ConfigMaps(expected.GetNamespace()).Create(ctx, &expected, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonata configmap %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataConfigMapCreated", expected.GetName())
	return created, nil
}

func (r *Reconciler) updateConfigMap(ctx context.Context, transform *eventing.EventTransform, expected corev1.ConfigMap) (*corev1.ConfigMap, error) {
	updated, err := r.k8s.CoreV1().ConfigMaps(expected.GetNamespace()).Update(ctx, &expected, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update configmap %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataConfigMapUpdated", expected.GetName())
	return updated, nil
}

func (r *Reconciler) deleteJsonataTransformationSinkBinding(ctx context.Context, transform *eventing.EventTransform) error {
	sbName := jsonataSinkBindingName(transform)
	_, err := r.jsonataSinkBindingLister.SinkBindings(transform.GetNamespace()).Get(sbName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get sink binding %s/%s: %w", transform.GetNamespace(), sbName, err)
	}

	err = r.client.SourcesV1().SinkBindings(transform.GetNamespace()).Delete(ctx, sbName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete sink binding %s/%s: %w", transform.GetNamespace(), sbName, err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataSinkBindingDeleted", sbName)
	return nil
}

func (r *Reconciler) createSinkBinding(ctx context.Context, transform *eventing.EventTransform, expected sources.SinkBinding) (*sources.SinkBinding, error) {
	created, err := r.client.SourcesV1().SinkBindings(expected.GetNamespace()).Create(ctx, &expected, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonata sink binding %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataSinkBindingCreated", expected.GetName())
	return created, nil
}

func (r *Reconciler) updateSinkBinding(ctx context.Context, transform *eventing.EventTransform, expected sources.SinkBinding) (*sources.SinkBinding, error) {
	updated, err := r.client.SourcesV1().SinkBindings(expected.GetNamespace()).Update(ctx, &expected, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update sink binding %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataSinkBindingUpdated", expected.GetName())
	return updated, nil
}

func (r *Reconciler) deleteJsonataTransformationCertificate(ctx context.Context, transform *eventing.EventTransform) error {
	certificate := jsonataCertificate(ctx, transform)
	_, err := r.cmCertificateLister.Certificates(certificate.GetNamespace()).Get(certificate.GetName())
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get certificate %s/%s: %w", certificate.GetNamespace(), certificate.GetName(), err)
	}

	err = r.cmClient.CertmanagerV1().Certificates(certificate.GetNamespace()).Delete(ctx, certificate.GetName(), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete certificate %s/%s: %w", certificate.GetNamespace(), certificate.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataCertificateDeleted", certificate.GetName())
	return nil
}

func (r *Reconciler) createCertificate(ctx context.Context, transform *eventing.EventTransform, expected *cmapis.Certificate) (*cmapis.Certificate, error) {
	created, err := r.cmClient.CertmanagerV1().Certificates(expected.GetNamespace()).Create(ctx, expected, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonata certificate %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataCertificateCreated", expected.GetName())
	return created, nil
}

func (r *Reconciler) updateCertificate(ctx context.Context, transform *eventing.EventTransform, expected *cmapis.Certificate) (*cmapis.Certificate, error) {
	updated, err := r.cmClient.CertmanagerV1().Certificates(expected.GetNamespace()).Update(ctx, expected, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update jsonata certificate %s/%s: %w", expected.GetNamespace(), expected.GetName(), err)
	}
	controller.GetEventRecorder(ctx).Event(transform, corev1.EventTypeNormal, "JsonataCertificateUpdated", expected.GetName())
	return updated, nil
}
