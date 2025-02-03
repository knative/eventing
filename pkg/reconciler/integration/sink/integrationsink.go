/*
Copyright 2024 The Knative Authors

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

package sink

import (
	"context"
	"fmt"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing/pkg/reconciler/integration/sink/resources"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/network"

	"knative.dev/eventing/pkg/apis/feature"

	"k8s.io/utils/ptr"
	sinks "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	eventingv1alpha1listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/eventingtls"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	sinkReconciled    = "IntegrationSinkReconciled"
	deploymentCreated = "DeploymentCreated"
	deploymentUpdated = "DeploymentUpdated"
	serviceCreated    = "ServiceCreated"
	serviceUpdated    = "ServiceUpdated"
)

type Reconciler struct {
	secretLister      corev1listers.SecretLister
	eventPolicyLister eventingv1alpha1listers.EventPolicyLister

	kubeClientSet kubernetes.Interface

	deploymentLister appsv1listers.DeploymentLister
	serviceLister    corev1listers.ServiceLister

	systemNamespace string
}

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason IntegrationSink.
func newReconciledNormal(namespace, name string) reconciler.Event {
	return reconciler.NewEvent(corev1.EventTypeNormal, sinkReconciled, "IntegrationSink reconciled: \"%s/%s\"", namespace, name)
}

func (r *Reconciler) ReconcileKind(ctx context.Context, sink *sinks.IntegrationSink) reconciler.Event {
	featureFlags := feature.FromContext(ctx)

	_, err := r.reconcileDeployment(ctx, sink)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling Pod", zap.Error(err))
		return err
	}

	_, err = r.reconcileService(ctx, sink)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling Service", zap.Error(err))
		return err
	}

	if err := r.reconcileAddress(ctx, sink); err != nil {
		return fmt.Errorf("failed to reconcile address: %w", err)
	}

	err = auth.UpdateStatusWithEventPolicies(featureFlags, &sink.Status.AppliedEventPoliciesStatus, &sink.Status, r.eventPolicyLister, sinks.SchemeGroupVersion.WithKind("IntegrationSink"), sink.ObjectMeta)
	if err != nil {
		return fmt.Errorf("could not update IntegrationSink status with EventPolicies: %v", err)
	}

	return newReconciledNormal(sink.Namespace, sink.Name)
}

func (r *Reconciler) reconcileDeployment(ctx context.Context, sink *sinks.IntegrationSink) (*v1.Deployment, error) {

	expected := resources.MakeDeploymentSpec(sink)
	deployment, err := r.deploymentLister.Deployments(sink.Namespace).Get(expected.Name)
	if apierrors.IsNotFound(err) {
		deployment, err = r.kubeClientSet.AppsV1().Deployments(sink.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating new Deployment: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(sink, corev1.EventTypeNormal, deploymentCreated, "Deployment created %q", deployment.Name)
	} else if err != nil {
		return nil, fmt.Errorf("getting Deployment: %v", err)
	} else if !metav1.IsControlledBy(deployment, sink) {
		return nil, fmt.Errorf("Deployment %q is not owned by IntegrationSink %q", deployment.Name, sink.Name)
	} else if r.podSpecChanged(deployment.Spec.Template.Spec, expected.Spec.Template.Spec) {

		deployment.Spec.Template.Spec = expected.Spec.Template.Spec
		deployment, err = r.kubeClientSet.AppsV1().Deployments(sink.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating Deployment: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(sink, corev1.EventTypeNormal, deploymentUpdated, "Deployment %q updated", deployment.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing Deployment", zap.Any("Deployment", deployment))
	}

	sink.Status.PropagateDeploymentStatus(&deployment.Status)
	return deployment, nil
}

func (r *Reconciler) reconcileService(ctx context.Context, sink *sinks.IntegrationSink) (*corev1.Service, error) {
	expected := resources.MakeService(sink)

	svc, err := r.serviceLister.Services(sink.Namespace).Get(expected.Name)
	if apierrors.IsNotFound(err) {
		svc, err := r.kubeClientSet.CoreV1().Services(sink.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating new Service: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(sink, corev1.EventTypeNormal, serviceCreated, "Service created %q", svc.Name)
	} else if err != nil {
		return nil, fmt.Errorf("getting Service : %v", err)
	} else if !metav1.IsControlledBy(svc, sink) {
		return nil, fmt.Errorf("Service %q is not owned by IntegrationSink %q", svc.Name, sink.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing Service", zap.Any("Service", svc))
	}

	return svc, nil
}

func (r *Reconciler) reconcileAddress(ctx context.Context, sink *sinks.IntegrationSink) error {

	featureFlags := feature.FromContext(ctx)
	if featureFlags.IsPermissiveTransportEncryption() {
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpAddress := r.httpAddress(sink)
		httpsAddress := r.httpsAddress(caCerts, sink)
		// Permissive mode:
		// - status.address http address with host-based routing
		// - status.addresses:
		//   - https address with path-based routing
		//   - http address with host-based routing
		sink.Status.Addresses = []duckv1.Addressable{httpsAddress, httpAddress}
		sink.Status.Address = &httpAddress
	} else if featureFlags.IsStrictTransportEncryption() {
		// Strict mode: (only https addresses)
		// - status.address https address with path-based routing
		// - status.addresses:
		//   - https address with path-based routing
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpsAddress := r.httpsAddress(caCerts, sink)
		sink.Status.Addresses = []duckv1.Addressable{httpsAddress}
		sink.Status.Address = &httpsAddress
	} else {
		httpAddress := r.httpAddress(sink)
		sink.Status.Address = &httpAddress
	}

	if featureFlags.IsOIDCAuthentication() {
		audience := auth.GetAudience(sinks.SchemeGroupVersion.WithKind("IntegrationSink"), sink.ObjectMeta)

		logging.FromContext(ctx).Debugw("Setting the audience", zap.String("audience", audience))
		sink.Status.Address.Audience = &audience
		for i := range sink.Status.Addresses {
			sink.Status.Addresses[i].Audience = &audience
		}
	} else {
		logging.FromContext(ctx).Debug("Clearing the imc audience as OIDC is not enabled")
		sink.Status.Address.Audience = nil
		for i := range sink.Status.Addresses {
			sink.Status.Addresses[i].Audience = nil
		}
	}

	sink.GetConditionSet().Manage(sink.GetStatus()).MarkTrue(sinks.IntegrationSinkConditionAddressable)

	return nil
}

func (r *Reconciler) getCaCerts() (*string, error) {
	// Getting the secret called "imc-dispatcher-tls" from system namespace
	secret, err := r.secretLister.Secrets(r.systemNamespace).Get(eventingtls.IntegrationSinkDispatcherServerTLSSecretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certs from %s/%s: %w", r.systemNamespace, eventingtls.IntegrationSinkDispatcherServerTLSSecretName, err)
	}
	caCerts, ok := secret.Data[eventingtls.SecretCACert]
	if !ok {
		return nil, nil
	}
	return ptr.To(string(caCerts)), nil
}

func (r *Reconciler) httpAddress(sink *sinks.IntegrationSink) duckv1.Addressable {
	// http address uses host-based routing
	httpAddress := duckv1.Addressable{
		Name: ptr.To("http"),
		URL: &apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname(sink.GetName()+"-deployment", sink.GetNamespace()),
		},
	}
	return httpAddress
}

func (r *Reconciler) httpsAddress(certs *string, sink *sinks.IntegrationSink) duckv1.Addressable {
	addr := r.httpAddress(sink)
	addr.URL.Scheme = "https"
	addr.CACerts = certs
	return addr
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
