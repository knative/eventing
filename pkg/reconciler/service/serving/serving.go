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

package serving

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/reconciler/service"
	"knative.dev/pkg/apis"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"
)

// ServiceReconciler reconciles addressable services with knative serving API.
type ServiceReconciler struct {
	ServingClientSet servingclientset.Interface
	ServingLister    servinglisters.ServiceLister
}

// Reconcile reconciles a service with knative serving API.
func (r *ServiceReconciler) Reconcile(ctx context.Context, owner metav1.OwnerReference, args service.Args) (*service.Status, error) {
	if err := service.ValidateArgs(args); err != nil {
		return nil, err
	}
	fillDefaults(&args, owner)

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

	existing, err := r.ServingLister.Services(args.ServiceMeta.Namespace).Get(args.ServiceMeta.Name)
	if apierrors.IsNotFound(err) {
		existing, err = r.ServingClientSet.ServingV1().Services(args.ServiceMeta.Namespace).Create(svc)
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
		existing, err = r.ServingClientSet.ServingV1().Services(args.ServiceMeta.Namespace).Update(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to update serving service: %w", err)
		}
	}

	return servingServiceStatus(existing), nil
}

// GetStatus returns the knative serving service status.
func (r *ServiceReconciler) GetStatus(ctx context.Context, svcMeta metav1.ObjectMeta) (*service.Status, error) {
	existing, err := r.ServingLister.Services(svcMeta.Namespace).Get(svcMeta.Name)
	if err != nil {
		return nil, err
	}
	return servingServiceStatus(existing), nil
}

func fillDefaults(args *service.Args, owner metav1.OwnerReference) {
	// Make sure the service metadata has proper owner reference.
	args.ServiceMeta.OwnerReferences = append(args.ServiceMeta.OwnerReferences, owner)
	// Serving service requires a strict prefix.
	// Add "-rev" to make it strict prefix.
	args.DeployMeta.Name = args.DeployMeta.Name + "-rev"

	// Always use cluster local service.
	if args.ServiceMeta.Labels == nil {
		args.ServiceMeta.Labels = make(map[string]string)
	}
	args.ServiceMeta.Labels["serving.knative.dev/visibility"] = "cluster-local"
}

func servingServiceStatus(svc *servingv1.Service) *service.Status {
	cond := svc.Status.GetCondition(apis.ConditionReady)
	ss := &service.Status{
		IsReady: svc.Status.IsReady(),
		URL:     svc.Status.URL,
	}
	if cond != nil {
		ss.Message = cond.Message
		ss.Reason = cond.Reason
	}
	return ss
}
