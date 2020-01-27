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

package kube

import (
	"context"
	"fmt"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/eventing/pkg/reconciler/service"
	"knative.dev/pkg/apis"
)

// ServiceReconciler reconciles addressable services implemented with
// k8s services and deployments.
type ServiceReconciler struct {
	KubeClientSet    kubernetes.Interface
	ServiceLister    corev1listers.ServiceLister
	DeploymentLister appsv1listers.DeploymentLister
}

var _ service.Reconciler = (*ServiceReconciler)(nil)

// Reconcile reconciles an addressable service with k8s service and deployment.
func (r *ServiceReconciler) Reconcile(ctx context.Context, owner metav1.OwnerReference, args service.Args) (*service.Status, error) {
	if err := service.ValidateArgs(args); err != nil {
		return nil, err
	}
	fillDefaults(&args, owner)

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
	rd, err := r.reconcileDeployment(ctx, d)
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
					TargetPort: intstr.FromInt(int(args.PodSpec.Containers[0].Ports[0].ContainerPort)),
				},
			},
		},
	}
	rsvc, err := r.reconcileService(ctx, svc)
	if err != nil {
		return nil, err
	}

	if duck.DeploymentIsAvailable(&rd.Status, true) {
		return &service.Status{
			IsReady: true,
			URL: &apis.URL{
				Scheme: "http",
				Host:   names.ServiceHostName(rsvc.Name, rsvc.Namespace),
			},
		}, nil
	}

	return &service.Status{
		IsReady: false,
		Reason:  "DeploymentUnavailable",
		Message: fmt.Sprintf("The Deployment %q is unavailable", rd.Name),
	}, nil
}

// GetStatus returns the status of a k8s service.
func (r *ServiceReconciler) GetStatus(ctx context.Context, svcMeta metav1.ObjectMeta) (*service.Status, error) {
	existing, err := r.ServiceLister.Services(svcMeta.Namespace).Get(svcMeta.Name)
	if err != nil {
		return nil, err
	}
	return &service.Status{
		IsReady: true,
		URL: &apis.URL{
			Scheme: "http",
			Host:   names.ServiceHostName(existing.Name, existing.Namespace),
		},
	}, nil
}

func fillDefaults(args *service.Args, owner metav1.OwnerReference) {
	// Make sure the service metadata has proper owner reference.
	args.ServiceMeta.OwnerReferences = append(args.ServiceMeta.OwnerReferences, owner)
	args.DeployMeta.OwnerReferences = append(args.DeployMeta.OwnerReferences, owner)
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
}

func (r *ServiceReconciler) reconcileDeployment(ctx context.Context, d *v1.Deployment) (*v1.Deployment, error) {
	existing, err := r.DeploymentLister.Deployments(d.Namespace).Get(d.Name)
	if apierrors.IsNotFound(err) {
		existing, err = r.KubeClientSet.AppsV1().Deployments(d.Namespace).Create(d)
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
		existing, err = r.KubeClientSet.AppsV1().Deployments(existing.Namespace).Update(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to update deployment: %w", err)
		}
	}
	return existing, nil
}

// reconcileService reconciles the K8s Service 'svc'.
func (r *ServiceReconciler) reconcileService(ctx context.Context, svc *corev1.Service) (*corev1.Service, error) {
	existing, err := r.ServiceLister.Services(svc.Namespace).Get(svc.Name)
	if apierrors.IsNotFound(err) {
		existing, err = r.KubeClientSet.CoreV1().Services(svc.Namespace).Create(svc)
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
		existing, err = r.KubeClientSet.CoreV1().Services(existing.Namespace).Update(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to update service: %w", err)
		}
	}
	return existing, nil
}
