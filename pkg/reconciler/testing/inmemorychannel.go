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

package testing

import (
	"context"
	"time"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	"github.com/knative/pkg/apis"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/types"
)

// InMemoryChannelOption enables further configuration of a InMemoryChannel.
type InMemoryChannelOption func(*v1alpha1.InMemoryChannel)

// NewInMemoryChannel creates an InMemoryChannel with InMemoryChannelOptions.
func NewInMemoryChannel(name, namespace string, imcopt ...InMemoryChannelOption) *v1alpha1.InMemoryChannel {
	imc := &v1alpha1.InMemoryChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.InMemoryChannelSpec{},
	}
	for _, opt := range imcopt {
		opt(imc)
	}
	imc.SetDefaults(context.Background())
	return imc
}

func WithInitInMemoryChannelConditions(imc *v1alpha1.InMemoryChannel) {
	imc.Status.InitializeConditions()
}

func WithInMemoryChannelDeleted(imc *v1alpha1.InMemoryChannel) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	imc.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithInMemoryChannelDeploymentNotReady(reason, message string) InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.MarkDispatcherFailed(reason, message)
	}
}

func WithInMemoryChannelDeploymentReady() InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.PropagateDispatcherStatus(&appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}})
	}
}

func WithInMemoryChannelServicetNotReady(reason, message string) InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.MarkServiceFailed(reason, message)
	}
}

func WithInMemoryChannelServiceReady() InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.MarkServiceTrue()
	}
}

func WithInMemoryChannelChannelServicetNotReady(reason, message string) InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.MarkChannelServiceFailed(reason, message)
	}
}

func WithInMemoryChannelChannelServiceReady() InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.MarkChannelServiceTrue()
	}
}

func WithInMemoryChannelEndpointsNotReady(reason, message string) InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.MarkEndpointsFailed(reason, message)
	}
}

func WithInMemoryChannelEndpointsReady() InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.MarkEndpointsTrue()
	}
}

func WithInMemoryChannelAddress(a string) InMemoryChannelOption {
	return func(imc *v1alpha1.InMemoryChannel) {
		imc.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   a,
		})
	}
}
