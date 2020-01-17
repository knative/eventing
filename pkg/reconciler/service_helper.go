/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconciler

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/kmeta"
	servingcommon "knative.dev/serving/pkg/apis/serving"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"
	servingclient "knative.dev/serving/pkg/client/injection/client"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"
)

type ServiceStatus struct {
	IsReady bool
	URL     *apis.URL
	Reason  string
	Message string
}

type ServiceArgs struct {
	ServiceMeta metav1.ObjectMeta
	DeployMeta  metav1.ObjectMeta
	PodSpec     corev1.PodSpec
}

type ServiceHelper struct {
	kubeClientSet    kubernetes.Interface
	servingClientSet servingclientset.Interface
	serviceLister    corev1listers.ServiceLister
	deploymentLister appsv1listers.DeploymentLister
	servingLister    servinglisters.ServiceLister
	APIChecker       APIChecker
}

func NewServiceHelper(ctx context.Context, deploymentLister appsv1listers.DeploymentLister, serviceLister corev1listers.ServiceLister, servingLister servinglisters.ServiceLister) *ServiceHelper {
	return &ServiceHelper{
		kubeClientSet:    kubeclient.Get(ctx),
		servingClientSet: servingclient.Get(ctx),
		deploymentLister: deploymentLister,
		serviceLister:    serviceLister,
		servingLister:    servingLister,
		APIChecker:       &apiCheckerImpl{kubeClientSet: kubeclient.Get(ctx)},
	}
}

type APIChecker interface {
	Exists(schema.GroupVersion) error
}

type apiCheckerImpl struct {
	kubeClientSet kubernetes.Interface
}

func (c *apiCheckerImpl) Exists(s schema.GroupVersion) error {
	return discovery.ServerSupportsVersion(c.kubeClientSet.Discovery(), servingv1.SchemeGroupVersion)
}

func (h *ServiceHelper) ServiceStatus(ctx context.Context, owner kmeta.OwnerRefable, serviceMeta metav1.ObjectMeta) (*ServiceStatus, error) {
	useServing, err := h.useServingService(ctx, owner)
	if err != nil {
		return nil, err
	}

	if useServing {
		existing, err := h.servingLister.Services(serviceMeta.Namespace).Get(serviceMeta.Name)
		if err != nil {
			return nil, err
		}
		return servingServiceStatus(existing), nil
	}

	existing, err := h.serviceLister.Services(serviceMeta.Namespace).Get(serviceMeta.Name)
	if err != nil {
		return nil, err
	}
	return &ServiceStatus{
		IsReady: true,
		URL: &apis.URL{
			Scheme: "http",
			Host:   names.ServiceHostName(existing.Name, existing.Namespace),
		},
	}, nil
}

func (h *ServiceHelper) ReconcileService(ctx context.Context, owner kmeta.OwnerRefable, args ServiceArgs) (*ServiceStatus, error) {
	if err := validateArgs(args); err != nil {
		return nil, err
	}

	useServing, err := h.useServingService(ctx, owner)
	if err != nil {
		return nil, err
	}

	// Make sure the service metadata has proper owner reference.
	args.ServiceMeta.OwnerReferences = append(args.ServiceMeta.OwnerReferences, *kmeta.NewControllerRef(owner))

	if useServing {
		return h.reconcileServingService(ctx, args)
	}

	return h.reconcileDeploymentService(ctx, owner, args)
}

func validateArgs(args ServiceArgs) error {
	if !strings.HasPrefix(args.DeployMeta.Name, args.ServiceMeta.Name) {
		return fmt.Errorf("service name must be a prefix of deployment name")
	}
	if err := servingcommon.ValidatePodSpec(args.PodSpec); err != nil {
		return err
	}
	if len(args.PodSpec.Containers[0].Ports) == 0 {
		args.PodSpec.Containers[0].Ports = []corev1.ContainerPort{
			{
				ContainerPort: 8080,
			},
		}
	}
	return nil
}

func (h *ServiceHelper) reconcileServingService(ctx context.Context, args ServiceArgs) (*ServiceStatus, error) {
	// Serving service requires a strict prefix.
	// Add "-rev" to make it strict prefix.
	args.DeployMeta.Name = args.DeployMeta.Name + "-rev"

	// Just for testing purpose.
	// if args.DeployMeta.Annotations == nil {
	// 	args.DeployMeta.Annotations = make(map[string]string)
	// }
	// args.DeployMeta.Annotations["autoscaling.knative.dev/minScale"] = "1"

	// Always use cluster local service.
	if args.ServiceMeta.Labels == nil {
		args.ServiceMeta.Labels = make(map[string]string)
	}
	args.ServiceMeta.Labels["serving.knative.dev/visibility"] = "cluster-local"

	svc := &servingv1.Service{
		ObjectMeta: args.ServiceMeta,
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					ObjectMeta: args.DeployMeta,
					Spec: servingv1.RevisionSpec{
						PodSpec: args.PodSpec,
					},
				},
			},
		},
	}

	existing, err := h.servingLister.Services(args.ServiceMeta.Namespace).Get(args.ServiceMeta.Name)
	if apierrors.IsNotFound(err) {
		existing, err = h.servingClientSet.ServingV1().Services(args.ServiceMeta.Namespace).Create(svc)
		if err != nil {
			return nil, fmt.Errorf("failed to create serving service: %w", err)
		}
		return servingServiceStatus(existing), nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get existing serving service: %w", err)
	}

	if !equality.Semantic.DeepDerivative(svc.Spec, existing.Spec) {
		desired := existing.DeepCopy()
		desired.Spec = svc.Spec
		existing, err = h.servingClientSet.ServingV1().Services(args.ServiceMeta.Namespace).Update(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to update serving service: %w", err)
		}
	}

	return servingServiceStatus(existing), nil
}

func servingServiceStatus(svc *servingv1.Service) *ServiceStatus {
	cond := svc.Status.GetCondition(apis.ConditionReady)
	ss := &ServiceStatus{
		IsReady: svc.Status.IsReady(),
		URL:     svc.Status.URL,
	}
	if cond != nil {
		ss.Message = cond.Message
		ss.Reason = cond.Reason
	}
	return ss
}

func (h *ServiceHelper) reconcileDeploymentService(ctx context.Context, owner kmeta.OwnerRefable, args ServiceArgs) (*ServiceStatus, error) {
	// Make sure the deployment metadata has proper owner reference.
	args.DeployMeta.OwnerReferences = append(args.DeployMeta.OwnerReferences, *kmeta.NewControllerRef(owner))
	args.PodSpec.Containers[0].Ports[0].Name = "http"
	userPort := args.PodSpec.Containers[0].Ports[0].ContainerPort

	if args.PodSpec.Containers[0].LivenessProbe != nil {
		if args.PodSpec.Containers[0].LivenessProbe.HTTPGet != nil {
			args.PodSpec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.FromInt(int(userPort))
		}
	}
	if args.PodSpec.Containers[0].ReadinessProbe != nil {
		if args.PodSpec.Containers[0].ReadinessProbe.HTTPGet != nil {
			args.PodSpec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(int(userPort))
		}
	}

	d := &v1.Deployment{
		ObjectMeta: args.DeployMeta,
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.DeployMeta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: args.DeployMeta.Labels,
				},
				Spec: args.PodSpec,
			},
		},
	}
	rd, err := h.reconcileDeployment(ctx, d)
	if err != nil {
		return nil, err
	}

	svc := &corev1.Service{
		ObjectMeta: args.ServiceMeta,
		Spec: corev1.ServiceSpec{
			Selector: args.DeployMeta.Labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(int(userPort)),
				},
			},
		},
	}
	rsvc, err := h.reconcileService(ctx, svc)
	if err != nil {
		return nil, err
	}

	if duck.DeploymentIsAvailable(&rd.Status, true) {
		return &ServiceStatus{
			IsReady: true,
			URL: &apis.URL{
				Scheme: "http",
				Host:   names.ServiceHostName(rsvc.Name, rsvc.Namespace),
			},
		}, nil
	}

	return &ServiceStatus{
		IsReady: false,
		Reason:  "DeploymentUnavailable",
		Message: fmt.Sprintf("The Deployment %q is unavailable", rd.Name),
	}, nil
}

func (h *ServiceHelper) reconcileDeployment(ctx context.Context, d *v1.Deployment) (*v1.Deployment, error) {
	existing, err := h.deploymentLister.Deployments(d.Namespace).Get(d.Name)
	if apierrors.IsNotFound(err) {
		existing, err = h.kubeClientSet.AppsV1().Deployments(d.Namespace).Create(d)
		if err != nil {
			return nil, fmt.Errorf("failed to create deployment: %w", err)
		}
		return existing, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get existing deployment: %w", err)
	}

	if !equality.Semantic.DeepDerivative(d.Spec, existing.Spec) {
		// Don't modify the informers copy.
		desired := existing.DeepCopy()
		desired.Spec = d.Spec
		existing, err = h.kubeClientSet.AppsV1().Deployments(existing.Namespace).Update(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to update deployment: %w", err)
		}
	}
	return existing, nil
}

// reconcileService reconciles the K8s Service 'svc'.
func (h *ServiceHelper) reconcileService(ctx context.Context, svc *corev1.Service) (*corev1.Service, error) {
	existing, err := h.serviceLister.Services(svc.Namespace).Get(svc.Name)
	if apierrors.IsNotFound(err) {
		existing, err = h.kubeClientSet.CoreV1().Services(svc.Namespace).Create(svc)
		if err != nil {
			return nil, fmt.Errorf("failed to create service: %w", err)
		}
		return existing, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get existing service: %w", err)
	}

	// spec.clusterIP is immutable and is set on existing services. If we don't set this to the same value, we will
	// encounter an error while updating.
	svc.Spec.ClusterIP = existing.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(svc.Spec, existing.Spec) {
		// Don't modify the informers copy.
		desired := existing.DeepCopy()
		desired.Spec = svc.Spec
		existing, err = h.kubeClientSet.CoreV1().Services(existing.Namespace).Update(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to update service: %w", err)
		}
	}
	return existing, nil
}

func (h *ServiceHelper) useServingService(ctx context.Context, owner kmeta.OwnerRefable) (bool, error) {
	if v, ok := owner.GetObjectMeta().GetAnnotations()["eventing.knative.dev/serviceFlavor"]; !ok && v != "knative" {
		return false, nil
	}

	if err := h.APIChecker.Exists(servingv1.SchemeGroupVersion); err != nil {
		if strings.Contains(err.Error(), "server does not support API version") {
			return false, err
		}
		return false, fmt.Errorf("failed to check serving API version: %w", err)
	}

	return true, nil
}
