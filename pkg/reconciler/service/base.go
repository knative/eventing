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

package service

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	servingcommon "knative.dev/serving/pkg/apis/serving"
)

const (
	// ServingFlavor is the Knative Serving service flavor.
	ServingFlavor = "knative"
)

// Status represents the status of an addressable service.
type Status struct {
	IsReady bool
	URL     *apis.URL
	Reason  string
	Message string
}

// Args is the arguments to reconcile an addressable service.
type Args struct {
	ServiceMeta metav1.ObjectMeta
	DeployMeta  metav1.ObjectMeta
	PodSpec     corev1.PodSpec
}

// Reconciler is the interface to reconcile addressable services.
type Reconciler interface {
	// Reconcile reconciles a service.
	Reconcile(context.Context, metav1.OwnerReference, Args) (*Status, error)
	// GetStatus get the status of a service.
	GetStatus(context.Context, metav1.ObjectMeta) (*Status, error)
}

// ValidateArgs validates the arguments for a service.
func ValidateArgs(args Args) error {
	if !strings.HasPrefix(args.DeployMeta.Name, args.ServiceMeta.Name) {
		return fmt.Errorf("service name must be a prefix of deployment name")
	}
	// Use serving's validation since it's more strict.
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
