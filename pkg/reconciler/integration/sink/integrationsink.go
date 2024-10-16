package sink

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
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

type Reconciler struct {
	secretLister      corev1listers.SecretLister
	eventPolicyLister eventingv1alpha1listers.EventPolicyLister

	kubeClientSet kubernetes.Interface

	deploymentLister appsv1listers.DeploymentLister
	serviceLister    corev1listers.ServiceLister
	podLister        corev1listers.PodLister

	systemNamespace string
}

func (r *Reconciler) ReconcileKind(ctx context.Context, sink *sinks.IntegrationSink) reconciler.Event {
	pod, err := r.reconcileDeployment(ctx, sink)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling Pod", zap.Error(err))
		return err
	}

	if pod != nil {
	}

	service, err := r.reconcileService(ctx, sink)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error reconciling Service", zap.Error(err))
		return err
	}

	if service != nil {
	}

	if err := r.reconcileAddress(ctx, sink); err != nil {
		return fmt.Errorf("failed to reconcile address: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileDeployment(ctx context.Context, sink *sinks.IntegrationSink) (*appsv1.Deployment, error) {

	expected := resources.MakeDeploymentSpec(sink)
	pod, err := r.deploymentLister.Deployments(sink.Namespace).Get(expected.Name)
	if apierrors.IsNotFound(err) {
		pod, err := r.kubeClientSet.AppsV1().Deployments(sink.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating new Deployment: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(sink, corev1.EventTypeNormal, "FIXME___reconciled", "Deployment created %q", pod.Name)
	} else if err != nil {
		return nil, fmt.Errorf("getting Deployment: %v", err)
	} else if !metav1.IsControlledBy(pod, sink) {
		return nil, fmt.Errorf("pod %q is not owned by KameletSink %q", pod.Name, sink.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing Deployment", zap.Any("Pod", pod))

	}

	return pod, nil
}

func (r *Reconciler) reconcileService(ctx context.Context, sink *sinks.IntegrationSink) (*corev1.Service, error) {
	expected := resources.MakeService(sink)

	svc, err := r.serviceLister.Services(sink.Namespace).Get(expected.Name)
	if apierrors.IsNotFound(err) {
		svc, err := r.kubeClientSet.CoreV1().Services(sink.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating new Service: %v", err)
		}
		controller.GetEventRecorder(ctx).Eventf(sink, corev1.EventTypeNormal, "FIXME___reconciled", "Service created %q", svc.Name)
	} else if err != nil {
		return nil, fmt.Errorf("getting Service : %v", err)
	} else if !metav1.IsControlledBy(svc, sink) {
		return nil, fmt.Errorf("service %q is not owned by KameletSink %q", svc.Name, sink.Name)
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
			//			Path:   fmt.Sprintf("/%s/%s", sink.GetNamespace(), sink.GetName()),
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
