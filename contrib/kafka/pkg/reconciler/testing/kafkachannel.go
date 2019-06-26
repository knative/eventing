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

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/knative/eventing/contrib/kafka/pkg/apis/messaging/v1alpha1"
	"knative.dev/pkg/apis"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaChannelOption enables further configuration of a KafkaChannel.
type KafkaChannelOption func(*v1alpha1.KafkaChannel)

// NewKafkaChannel creates an KafkaChannel with KafkaChannelOptions.
func NewKafkaChannel(name, namespace string, ncopt ...KafkaChannelOption) *v1alpha1.KafkaChannel {
	nc := &v1alpha1.KafkaChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.KafkaChannelSpec{},
	}
	for _, opt := range ncopt {
		opt(nc)
	}
	nc.SetDefaults(context.Background())
	return nc
}

func WithInitKafkaChannelConditions(nc *v1alpha1.KafkaChannel) {
	nc.Status.InitializeConditions()
}

func WithKafkaChannelDeleted(nc *v1alpha1.KafkaChannel) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	nc.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithKafkaChannelTopicReady() KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkTopicTrue()
	}
}

func WithKafkaChannelDeploymentNotReady(reason, message string) KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkDispatcherFailed(reason, message)
	}
}

func WithKafkaChannelDeploymentReady() KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.PropagateDispatcherStatus(&appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}})
	}
}

func WithKafkaChannelServicetNotReady(reason, message string) KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkServiceFailed(reason, message)
	}
}

func WithKafkaChannelServiceReady() KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkServiceTrue()
	}
}

func WithKafkaChannelChannelServicetNotReady(reason, message string) KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkChannelServiceFailed(reason, message)
	}
}

func WithKafkaChannelChannelServiceReady() KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkChannelServiceTrue()
	}
}

func WithKafkaChannelEndpointsNotReady(reason, message string) KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkEndpointsFailed(reason, message)
	}
}

func WithKafkaChannelEndpointsReady() KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.MarkEndpointsTrue()
	}
}

func WithKafkaChannelAddress(a string) KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		nc.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   a,
		})
	}
}

func WithKafkaFinalizer(finalizerName string) KafkaChannelOption {
	return func(nc *v1alpha1.KafkaChannel) {
		finalizers := sets.NewString(nc.Finalizers...)
		finalizers.Insert(finalizerName)
		nc.SetFinalizers(finalizers.List())
	}
}
